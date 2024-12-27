package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	serverAddr, err := net.ResolveUDPAddr("udp", "localhost:8080")
	if err != nil {
		panic(err)
	}
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("CONNECT " + username))

	if err != nil {
		panic(err)
	}
	go func() {
		for {
			buffer := make([]byte, 1024)
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				fmt.Println("Error reading:", err)
				return
			}
			fmt.Println(string(buffer[:n]))
		}
	}()

	for {
		fmt.Print("> ")
		commands, _ := reader.ReadString('\n')
		commands = strings.TrimSpace(commands)

		_, err = conn.Write([]byte(commands))
		if err != nil {
			panic(err)
		}

		if commands == "DISCONNECT" {
			fmt.Println("Disconnected from server.")
			return
		}
	}
}
