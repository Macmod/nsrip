package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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

var progress int32
var total int32

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
	banner := `                 _
                (_)
  _ __  ___ _ __ _ _ __
 | '_ \/ __| '__| | '_ \
 | | | \__ \ |  | | |_) |
 |_| |_|___/_|  |_| .__/
                  | |
                  |_|
	`
	version := "1.0.1"

	var cloudProvider string
	var domain string
	var domainsFile string
	var numWorkers int
	var nameservers []string
	var quietMode bool
	var verboseMode bool
	var outputFile string

	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	pflag.StringVarP(&domain, "domain", "d", "", "Specify the target domain")
	pflag.StringVarP(&domainsFile, "list", "l", "", "Specify a file with a list of target domains")
	pflag.StringVarP(&cloudProvider, "nameservers", "n", "cloud", "Specify the nameserver list to use (aws, azure, gcp, cloud, or the path to a custom file)")
	pflag.IntVarP(&numWorkers, "workers", "w", 10, "Specify the number of workers")
	pflag.BoolVarP(&quietMode, "quiet", "q", false, "Only output raw results")
	pflag.BoolVarP(&verboseMode, "verbose", "v", false, "Verbose mode")
	pflag.StringVarP(&outputFile, "output", "o", "", "Output file where to save results")

	pflag.Parse()

	if !quietMode {
		fmt.Println(banner)
		fmt.Printf("[v%s]\n\n", version)
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
	sampleSize := numDomains * numNameservers

	if !quietMode {
		fmt.Printf("[+] %d domains x %d nameservers = %d queries\n", numDomains, numNameservers, sampleSize)
		fmt.Printf("[+] Workers: %d\n", numWorkers)
		fmt.Printf("[~] Mapping IPs for nameservers\n")
		fmt.Printf("[~] Press enter at any time to check the progress\n")
	}

	go func() {
		reader := bufio.NewReader(os.Stdin)

		for {
			_, err := reader.ReadString('\n')
			if err != nil {
				continue
			}

			val1 := atomic.LoadInt32(&progress)
			val2 := atomic.LoadInt32(&total)
			log.Printf(
				"[~] Progress: %d/%d (%.2f%%)\n",
				val1, val2,
				float64(val1)*100/float64(val2),
			)
		}
	}()

	mappedNameservers := resolveNameservers(nameservers, numWorkers)

	if !quietMode {
		fmt.Printf("[~] Querying domains against nameservers\n")
	}

	atomic.StoreInt32(&progress, int32(0))
	atomic.StoreInt32(&total, int32(sampleSize))

	type wrappedAnswer struct {
		nsIP   string
		domain string
		answer *dns.Msg
	}

	var wg sync.WaitGroup
	var wg2 sync.WaitGroup

	pendingQueries := make(chan string)      // Input
	queryAnswers := make(chan wrappedAnswer) // Output

	var fHandle *os.File
	var err error
	if outputFile != "" {
		fHandle, err = os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			log.Fatalf("[-] Could not create output file: %s", err)
		}
	}

	// Show results as they arrive
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		for result := range queryAnswers {
			var outputStr string

			domain := result.domain
			nsIP := result.nsIP
			nsName := mappedNameservers[nsIP]
			resp := result.answer
			if len(resp.Answer) > 0 {
				for _, answer := range resp.Answer {
					if aRecord, ok := answer.(*dns.A); ok {
						outputStr = fmt.Sprintf("[%s (%s)] %s => %s\n", nsName, nsIP, domain, aRecord.A)
						if fHandle != nil {
							fmt.Fprintf(fHandle, outputStr)
						}
						green.Printf(outputStr)
					} else if cnameRecord, ok := answer.(*dns.CNAME); ok {
						outputStr = fmt.Sprintf("[%s (%s)] %s => %s\n", nsName, nsIP, domain, cnameRecord.Target)
						if fHandle != nil {
							fmt.Fprintf(fHandle, outputStr)
						}
						green.Printf(outputStr)
					} else if aaaaRecord, ok := answer.(*dns.AAAA); ok {
						outputStr = fmt.Sprintf("[%s (%s)] %s => %s\n", nsName, nsIP, domain, aaaaRecord.AAAA)
						if fHandle != nil {
							fmt.Fprintf(fHandle, outputStr)
						}
						green.Printf(outputStr)
					} else {
						outputStr = fmt.Sprintf("[%s (%s)] %s\n", nsName, nsIP, domain, answer)
						if fHandle != nil {
							fmt.Fprintf(fHandle, outputStr)
						}
						yellow.Printf(outputStr)
					}
				}
			}
		}
	}()

	// Launch workers that run queries
	for i := 0; i < numWorkers; i++ {
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
				atomic.AddInt32(&progress, int32(1))

				if err != nil {
					if verboseMode {
						red.Printf(fmt.Sprintf("[-] %s\n", err))
					}
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

	wg2.Wait()
}
