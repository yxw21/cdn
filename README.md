Get CDN IP Range

```golang
package main

import (
	"fmt"
	"github.com/yxw21/cdn"
	"net"
)

func main() {
	fmt.Println(cdn.QueryName(net.ParseIP("172.67.186.220")))
	cdnNames := []string{cdn.Akamai}
	for _, name := range cdnNames {
		provider, err := cdn.GetProvider(name)
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
```