package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
)

func read(conn *net.Conn) {
	reader := bufio.NewReader(*conn)
	for {
		msg, _ := reader.ReadString('\n')
		fmt.Println(msg)
	}
}

func write(conn *net.Conn) {
	stdin := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Enter text: ")
		text, _ := stdin.ReadString('\n')
		fmt.Fprintf(*conn, text)
	}
}

func main() {
	// Get the server address and port from the commandline arguments.
	addrPtr := flag.String("ip", "54.88.236.161:8030", "IP:port string to connect to")
	flag.Parse()

	conn, _ := net.Dial("tcp", *addrPtr)

	go read(&conn)
	go write(&conn)

	for {
	}

	//TODO Try to connect to the server
	//TODO Start asynchronously reading and displaying messages
	//TODO Start getting and sending user messages.
}
