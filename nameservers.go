package main

import (
	"fmt"
	"net"
	"sync"
)

func resolveNameserver(ns string) (string, error) {
	ips, err := net.LookupIP(ns)
	if err != nil {
		return "", fmt.Errorf("Failed to resolve nameserver: %v", err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("No IP addresses found for nameserver: %s", ns)
	}
	return ips[0].String(), nil
}

func worker(id int, jobs <-chan string, results chan<- map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	for ns := range jobs {
		ip, err := resolveNameserver(ns)
		if err != nil {
			results <- map[string]string{ns: ""}
		} else {
			results <- map[string]string{ns: ip}
		}
	}
}

func resolveNameservers(nameservers []string, numWorkers int) map[string]string {
	resultsMap := make(map[string]string)

	jobs := make(chan string, len(nameservers))
	results := make(chan map[string]string, len(nameservers))

	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, jobs, results, &wg)
	}

	for _, ns := range nameservers {
		jobs <- ns
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		for ns, ip := range result {
			resultsMap[ip] = ns
		}
	}

	return resultsMap
}
