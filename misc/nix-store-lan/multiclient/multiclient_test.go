package multiclient

import (
	"fmt"
	"testing"
)

func TestMultiClient(t *testing.T) {

	c := MultiClient{}
	c.AddBackend("https://example.com")
	c.AddBackend("https://cache.nixos.org")

	resp, err := c.Get("https://testing.com/nix-cache-info")
	fmt.Println("RESP", resp, err)

}
