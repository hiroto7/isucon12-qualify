package isuports

import (
	"net/http"
	_ "net/http/pprof"
	"os"
)

// Profiling is opt-in and bound only to the guest's loopback interface.
func init() {
	if os.Getenv("ISUCON_PROFILE") == "1" {
		go func() { _ = http.ListenAndServe("127.0.0.1:6060", nil) }()
	}
}
