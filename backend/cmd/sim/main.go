package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"

	simv1 "github.com/alcares/mmoserver/backend/gen/go/sim/v1"
	"github.com/alcares/mmoserver/backend/internal/sim"
	"google.golang.org/grpc"
)

// The sim binary. It serves the batched env service over gRPC for the Python training pipeline
func main() {
	addr := flag.String("addr", "unix:///tmp/mmo-sim.sock", "listen address: unix:// path or host:port")
	flag.Parse()

	network, address := "tcp", *addr
	if path, ok := strings.CutPrefix(*addr, "unix://"); ok {
		// A crashed run leaves the socket file behind and Listen would fail on it
		network, address = "unix", path
		os.Remove(address)
	}

	lis, err := net.Listen(network, address)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}

	srv := grpc.NewServer()
	simv1.RegisterEnvServer(srv, sim.NewServer())

	log.Printf("sim serving on %s", *addr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
