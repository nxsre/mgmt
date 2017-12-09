package ladon

import (
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jpillora/ipfilter"
	"github.com/ory/ladon"
)

type IPFilterCondition struct {
	CIDRs string `json:"cidrs"`
}

func (c *IPFilterCondition) GetName() string {
	return "IPFilterCondition"
}

func (c *IPFilterCondition) Fulfills(value interface{}, r *ladon.Request) bool {
	ips, ok := value.(string)
	if !ok {
		return false
	}

	filter, err := ipfilter.New(ipfilter.Options{
		AllowedIPs:     strings.Split(c.CIDRs, ","),
		BlockByDefault: true,
	})
	if err != nil {
		fmt.Println(err)
		return false
	}

	cips := strings.Split(ips, ",")
	for _, ip := range cips {
		if !filter.Allowed(ip) {
			if js, err := json.Marshal(r); err == nil {
				fmt.Printf("%v 存在未授权IP %s\n", string(js), ip)
			}
			return false
		}
	}
	return true
}
