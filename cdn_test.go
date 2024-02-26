package cdn

import (
	"fmt"
	"net"
	"testing"
)

func TestCDN(t *testing.T) {
	fmt.Println(QueryName(net.ParseIP("172.67.186.220")))
	cdnNames := []string{Quic}
	for _, name := range cdnNames {
		provider, err := GetProvider(name)
		if err != nil {
			fmt.Println(err)
			continue
		}
		ipRanges, err := provider.FetchIPRanges()
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("%s IP ranges: %v\n", name, ipRanges)
	}
}
