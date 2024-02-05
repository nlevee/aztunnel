package tunnel

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/nlevee/aztunnel/internal/handler"
	"golang.org/x/crypto/ssh"
)

func RunTunnel(config *ssh.ClientConfig, lport, sshPort int, dest string, tunnelHandler handler.TunnelHandler) error {
	maxAttenmpts := 10
	attemptsLeft := maxAttenmpts
	var (
		client *ssh.Client
		err    error
	)
	for {
		client, err = ssh.Dial("tcp", fmt.Sprintf("localhost:%d", sshPort), config)
		if err != nil {
			attemptsLeft--
			if attemptsLeft <= 0 {
				return fmt.Errorf("failed to dial: %w", err)
			}
			time.Sleep(1 * time.Second)
			log.Printf("server dial error: %v: attempt %d/%d", err, maxAttenmpts-attemptsLeft, maxAttenmpts)
		} else {
			break
		}
	}
	defer client.Close()

	log.Printf("opening SSH connection to 'localhost:%d' succeed", sshPort)

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", lport))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer listener.Close()

	forwardedPort := listener.Addr().(*net.TCPAddr).Port
	log.Printf("waiting on 'localhost:%v' succeed", forwardedPort)

	// try to handle connection enabled
	if tunnelHandler != nil {
		if err := tunnelHandler.Handle(listener); err != nil {
			return fmt.Errorf("cannot handle after connection enabled: %w", err)
		}
	}

	for {
		// Like ssh -L by default, local connections are handled one at a time.
		// While one local connection is active in forward, others will be stuck
		// dialing, waiting for this Accept.
		local, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		// Issue a dial to the remote server on our SSH client
		// refers to the remote server.
		remote, err := client.Dial("tcp", dest)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("tunnel established with: %s", local.LocalAddr())
		go forward(local, remote)
	}
}

func forward(local, remote net.Conn) {
	defer local.Close()
	defer remote.Close()
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(local, remote)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(remote, local)
		done <- struct{}{}
	}()

	<-done
}
