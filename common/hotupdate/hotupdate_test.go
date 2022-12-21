package hotupdate

import (
	"fmt"
	"github.com/c3b2a7/goproxy/constant"
	"testing"
)

func TestStartService(t *testing.T) {
	constant.Version = "v13.0.0"
	startService(func(newVersion string) {
		fmt.Printf("\n[*] New version(%s) avaliable, restart services for update...\n", newVersion)
	})
}
