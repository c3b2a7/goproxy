package hotupdate

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/c3b2a7/goproxy/constant"
	"github.com/c3b2a7/goproxy/utils"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	latestVersion = "https://download.hitoor.com/goproxy/test/LatestVersion"
	downloadURL   = "https://download.hitoor.com/goproxy/test/proxy-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
)

var (
	networkUnavailable = errors.New("network unavailable")
	serviceOnce        = new(sync.Once)
)

type Version struct {
	Major string // 主版本号
	Minor string // 次版本号
	Patch string // 修订号
}

func (v Version) GetDownloadURL() string {
	return downloadURL
}

func (v Version) String() string {
	return "v" + strings.Join([]string{v.Major, v.Minor, v.Patch}, ".")
}

func NewInstance(version string) (v Version, err error) {
	_, err = fmt.Sscanf(version, "v%s", &version)
	if err != nil {
		return
	}
	splitN := strings.SplitN(version, ".", 3)
	if len(splitN) > 0 {
		v.Major = splitN[0]
	}
	if len(splitN) > 1 {
		v.Minor = splitN[1]
	}
	if len(splitN) > 2 {
		v.Patch = splitN[2]
	}
	return
}

func getLatestVersion() (v Version, err error) {
	resp, err := sendRequest(latestVersion)
	if err != nil || resp.StatusCode != 200 {
		return
	}
	bytes, _ := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	v, _ = NewInstance(string(bytes))
	return
}

func shouldUpdate() bool {
	if constant.Version == "unknown version" {
		return false
	}
	current, _ := NewInstance(constant.Version)
	latest, err := getLatestVersion()
	if err != nil {
		return false
	}
	if latest.Major == current.Major {
		if latest.Minor > current.Minor {
			return true
		}
		if latest.Minor == current.Minor &&
			latest.Patch > current.Patch {
			return true
		}
	}
	return false
}

func sendRequest(url string) (*http.Response, error) {
	laddr := utils.GetAvailableIfaceAddr()
	if laddr == "" {
		return nil, networkUnavailable
	}
	transport := utils.GetTransport(laddr)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return transport.RoundTrip(request)
}

func StartService(restart func(string)) {
	serviceOnce.Do(func() {
		go startService(restart)
	})
}

func startService(restart func(string)) {
	loop := func() {
		if shouldUpdate() {
			latest, err := getLatestVersion()
			if err != nil {
				return
			}
			time.Sleep(15 * time.Second)
			response, err := sendRequest(latest.GetDownloadURL())
			if err != nil || response.StatusCode != 200 {
				return
			}
			defer response.Body.Close()
			gzipReader, err := gzip.NewReader(response.Body)
			if err != nil {
				return
			}
			defer gzipReader.Close()

			tarReader := tar.NewReader(gzipReader)
			next, err := tarReader.Next()
			if err != nil {
				return
			}
			if next.Typeflag == tar.TypeReg {
				err = os.Remove(os.Args[0])
				if err != nil {
					log.Printf("remove file err: %v", err)
					return
				}
				file, err := os.OpenFile(os.Args[0], os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
				if err != nil {
					log.Printf("open file err: %v", err)
					return
				}
				defer file.Close()
				io.Copy(file, tarReader)
				restart(latest.String())
				constant.Version = latest.String()
			}
		}
	}
	for {
		time.Sleep(15 * time.Second)
		loop()
	}
}
