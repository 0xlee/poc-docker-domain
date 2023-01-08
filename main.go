package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

const topLevelDomain = "docker."

var port = 5354

// Default http client which connects to /var/run/docker.sock instead of network.
var httpClient = http.Client{
	Transport: &http.Transport{
		DialContext: func(_ context.Context, _ string, _ string) (net.Conn, error) {
			return net.Dial("unix", "/var/run/docker.sock")
		},
	},
}

func main() {
	// attach request handler func
	dns.HandleFunc(topLevelDomain, handleDnsRequest)

	// start dns server
	server := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}
	log.Printf("Starting at %d\n", port)
	err := server.ListenAndServe()
	defer server.Shutdown()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n ", err.Error())
	}
}

// DNS request handler
func handleDnsRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		for _, q := range m.Question {
			switch q.Qtype {
			// Only handle A records. Other records are not handled.
			case dns.TypeA:
				records, err := queryDockerContainers()
				if err != nil {
					log.Println(err)
				} else {
					ips := records[q.Name]
					for _, ip := range ips {
						rr, err := dns.NewRR(fmt.Sprintf("%s A %s\n", q.Name, ip))
						if err == nil {
							m.Answer = append(m.Answer, rr)
						}
					}
				}
			}
		}
	}

	w.WriteMsg(m)
}

// unused fields are omitted here
type Container struct {
	Names      []string
	HostConfig struct {
		NetworkMode string
	}
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string
		}
	}
}

// Queries container and network names and build a map from domain name to ip addresses.
// Didn't optimize for performance.
func queryDockerContainers() (map[string][]string, error) {
	resp, err := httpClient.Get("http://localhost/containers/json")
	if err != nil {
		return nil, fmt.Errorf("Error getting response: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %w", err)
	}

	var containers []Container
	err = json.Unmarshal(body, &containers)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling json response: %w", err)
	}

	result := map[string][]string{}
	for _, container := range containers {
		name := container.Names[0] // Assumes every docker container has at least one name and has in most cases one name
		for networkName, network := range container.NetworkSettings.Networks {
			normalizedNetworkName := normalizeNetworkName(networkName)
			ip := network.IPAddress

			hostNames := normalizeHostName(name, normalizedNetworkName)
			for _, hostName := range hostNames {
				s := []string{hostName}
				if normalizedNetworkName != "" {
					s = append(s, normalizedNetworkName)
				}
				s = append(s, topLevelDomain)

				domain := strings.Join(s, ".")
				result[domain] = append(result[domain], ip)
			}
		}
	}
	return result, nil
}

// Network names with suffix _default are trimmed.
// Assumes default network name is `bridge`.
func normalizeNetworkName(networkName string) string {
	const suffix = "_default"
	if strings.HasSuffix(networkName, suffix) {
		networkName = strings.TrimSuffix(networkName, suffix)
	}

	if networkName == "bridge" {
		return ""
	}

	return networkName
}

var hostNumberSuffixPattern = regexp.MustCompile(`-[0-9]+$`)

// In most cases, the hostname is of pattern "/<network-name>-<container-name>-<seq>"
// and we want to extract only the <container-name> and <container-name>-<seq>
func normalizeHostName(hostName string, normalizedNetworkName string) []string {
	hostName = strings.TrimPrefix(hostName, "/")
	hostName = strings.TrimPrefix(hostName, normalizedNetworkName)
	hostName = strings.TrimPrefix(hostName, "-")

	hostNameWithoutNumberSuffix := hostNumberSuffixPattern.ReplaceAllString(hostName, "")

	if hostName == hostNameWithoutNumberSuffix {
		return []string{hostName}
	} else {
		return []string{hostName, hostNameWithoutNumberSuffix}
	}
}
