package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/pflag"

	"github.com/miekg/dns"
)

var validProviders = map[string][]string{
	"aws":   []string{"nslists/aws.txt"},
	"azure": []string{"nslists/azure.txt"},
	"gcp":   []string{"nslists/gcp.txt"},
	"cloud": []string{"nslists/aws.txt", "nslists/azure.txt", "nslists/gcp.txt"},
}

func queryDNS(domain string, nameserver string) (*dns.Msg, error) {
	c := new(dns.Client)
	c.Timeout = 5 * time.Second
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeA)

	r, _, err := c.Exchange(m, nameserver)
	if err != nil {
		return nil, err
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("No answer from nameserver: %s", nameserver)
	}

	return r, nil
}

func main() {
	banner := `
                                  _                       
                                 (_)                      
  _ __   __ _ _ __ ___   ___ _ __ _ _ __  _ __   ___ _ __ 
 | '_ \ / _` + "`" + ` | '_ ` + "`" + ` _ \ / _ \ '__| | '_ \| '_ \ / _ \ '__|
 | | | | (_| | | | | | |  __/ |  | | |_) | |_) |  __/ |   
 |_| |_|\__,_|_| |_| |_|\___|_|  |_| .__/| .__/ \___|_|   
                                   | |   | |              
                                   |_|   |_|              
	`

	var cloudProvider string
	var domain string
	var domainsFile string
	var numWorkers int
	var nameservers []string
	var quietMode bool

	green := color.New(color.FgGreen)
	//red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	pflag.StringVarP(&domain, "domain", "d", "", "Specify the target domain")
	pflag.StringVarP(&domainsFile, "list", "l", "", "Specify a file with a list of target domains")
	pflag.StringVarP(&cloudProvider, "provider", "p", "cloud", "Specify the nameserver list to use (aws, azure, gcp, cloud, or the path to a custom file)")
	pflag.IntVarP(&numWorkers, "workers", "w", 5, "Specify the number of workers")
	pflag.BoolVarP(&quietMode, "quiet", "q", false, "Only output raw results")

	pflag.Parse()

	if !quietMode {
		fmt.Println(banner)
	}

	providerLists, ok := validProviders[cloudProvider]
	if !ok {
		providerLists = []string{cloudProvider}
	}

	if numWorkers <= 0 {
		log.Fatalf("Invalid number of workers: %d. It must be a positive integer.", numWorkers)
	}

	for _, filename := range providerLists {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("Error opening file %s: %v", filename, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			nameservers = append(nameservers, line)
		}

		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading file %s: %v", filename, err)
		}
	}

	var domainsList []string
	if domainsFile != "" {
		file, err := os.Open(domainsFile)
		if err != nil {
			log.Fatalf("Error opening file %s: %v", domainsFile, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			domainsList = append(domainsList, line)
		}

		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading file %s: %v", domainsFile, err)
		}
	} else {
		if domain == "" {
			log.Fatalf("You must provide either a domain (-d) or a list of domains (-l)")
		}

		domainsList = []string{domain}
	}

	numNameservers := len(nameservers)
	numDomains := len(domainsList)

	if !quietMode {
		fmt.Printf("[+] %d domains x %d nameservers = %d queries\n", numDomains, numNameservers, numDomains*numNameservers)
		fmt.Printf("[+] Workers: %d\n", numWorkers)
		fmt.Printf("[~] Mapping IPs for nameservers\n")
	}

	mappedNameservers := resolveNameservers(nameservers, numWorkers)

	if !quietMode {
		fmt.Printf("[~] Querying domains against nameservers\n")
	}

	type wrappedAnswer struct {
		nsIP   string
		domain string
		answer *dns.Msg
	}

	var wg sync.WaitGroup
	pendingQueries := make(chan string, numWorkers)      // Input
	queryAnswers := make(chan wrappedAnswer, numWorkers) // Output

	// Show results as they arrive
	go func() {
		for result := range queryAnswers {
			domain := result.domain
			nsIP := result.nsIP
			nsName := mappedNameservers[nsIP]
			resp := result.answer
			if len(resp.Answer) > 0 {
				for _, answer := range resp.Answer {
					if aRecord, ok := answer.(*dns.A); ok {
						green.Printf("[%s (%s)] %s => %s\n", nsName, nsIP, domain, aRecord.A)
					} else if cnameRecord, ok := answer.(*dns.CNAME); ok {
						green.Printf("[%s (%s)] %s => %s\n", nsName, nsIP, domain, cnameRecord.Target)
					} else if aaaaRecord, ok := answer.(*dns.AAAA); ok {
						green.Printf("[%s (%s)] %s => %s\n", nsName, nsIP, domain, aaaaRecord.AAAA)
					} else {
						yellow.Printf("[%s (%s)] %s\n", nsName, nsIP, domain, answer)
					}
				}
			}
		}
	}()

	// Launch workers that run queries
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for query := range pendingQueries {
				parts := strings.Split(query, "|")
				if len(parts) != 2 {
					continue
				}

				domain, nsIP := parts[0], parts[1]

				resp, err := queryDNS(domain, nsIP+":53")
				if err != nil {
					continue
				}

				queryAnswers <- wrappedAnswer{
					nsIP,
					domain,
					resp,
				}
			}
		}()
	}

	// Dispatch queries to be run by the workers
	for nsIP, _ := range mappedNameservers {
		if nsIP == "" {
			continue
		}

		for _, domain := range domainsList {
			query := fmt.Sprintf("%s|%s", domain, nsIP)
			pendingQueries <- query
		}
	}
	close(pendingQueries)

	wg.Wait()
	close(queryAnswers)
}
