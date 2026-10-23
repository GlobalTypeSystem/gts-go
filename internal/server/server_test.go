package server

import (
	"bytes"
	"log"
	"net"
	"testing"

	"github.com/GlobalTypeSystem/gts-go/gts"
)

func TestServerStartFailsWhenIPv4PortIsInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	var output bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(oldOutput)

	port := listener.Addr().(*net.TCPAddr).Port
	s := NewServer(gts.NewGtsStore(nil), "127.0.0.1", port, 0)
	if err := s.Start(); err == nil {
		t.Fatal("Start() succeeded with an occupied port")
	}
	if output.Len() != 0 {
		t.Fatalf("Start() logged a successful startup before binding: %s", output.String())
	}
}
