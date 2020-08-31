package main

import (
	"fmt"
	// "github.com/urfave/cli"
	"os"
	"time"
	"github.com/hashicorp/mdns"
)

func main() {
	/*
	app := cli.NewApp()
	app.Run(os.Argv)
	*/
	host, _ := os.Hostname()
	info := []string{"My awesome service"}
	service, err := mdns.NewMDNSService(host, "_foobar._tcp", "", "", 8000, nil, info)
	fmt.Println("service", service, err)
	if err != nil {
		return
	}

	// Create the mDNS server, defer shutdown
	config := &mdns.Config{
		Zone: service,
		Iface: nil, // TODO: bind to specific *net.Interface
		LogEmptyResponses: true,
	}
	server, err := mdns.NewServer(config)
	fmt.Println("server", server, err)
	if err != nil {
		return
	}
	defer server.Shutdown()

	for {
		entriesCh := make(chan *mdns.ServiceEntry, 4)
		go func() {
			for entry := range entriesCh {
				fmt.Printf("Got new entry: %+v\n", entry)
			}
			fmt.Println("No new entry")
		}()

		// Start the lookup
		mdns.Lookup("_foobar._tcp", entriesCh)
		time.Sleep(5 * time.Second)
		close(entriesCh)
	}
}
