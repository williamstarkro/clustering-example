package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
)

type Node struct {
	ID     int
	Leader bool
	Port   int
	Conn   net.Conn
}

var (
	max_nodes = 5
	nodeList = make(map[net.Conn]Node)
	leader = false
)

// we need to make channels in order to account for wait times in getting responses
// concurrent printer of new information
func printNewInfo(info chan string) {

	// outer bound for loop to always be running
	for {
		// selection of concurrent info
		select {
		case in := <-info:
			{
				fmt.Println(in)
			}
		}
	}
}

// whenever someone pings initializer TCP. Concurrent
func connectionCheck(conn net.Conn, info chan string, ID int) {

	// basically just need to make sure we get some sort of response other than an error
	reader := bufio.NewReader(conn)
	// open for loop to constantly be checking for errors
	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			// report loss of the connection
			info <- "Node:" + strconv.Itoa(ID) + " has disconnected."
			// end our check for this connection
			return
		}
	}
}

// send anything that nodes will understand as they are now ready to elect leaders and participate
// announce to each node all the other nodes
func sendInfo(conn net.Conn) {

	for _, node := range nodeList {
		if conn != node.Conn {
			io.WriteString(conn, strconv.Itoa(node.ID) + ":" + strconv.Itoa(node.Port) + ":" + strconv.FormatBool(node.Leader) + "\n")
		}
	}
	// let node that everyone has seen their presence
	io.WriteString(conn, "OK\n")
}


// Initializer
// creates a listener at 127.0.0.1:8001, that will constantly monitor for Nodes joining network
// Only accepts 5 nodes, after that, it will send out a message confirming that the network is 
// ready for a leader election. IDs are random numbers chosen using rand lib. Port will be equal
// to ID just to make sure we don't run into situation of non-unique IDs
func main() {
	// init our channel new info listener
	info := make(chan string)
	go printNewInfo(info)
	// open up listener
	listen, err := net.Listen("tcp", "127.0.0.1:8001")
	if err != nil {
		// fatal error starting up listener. Someone is occupying Port
		log.Fatal(err)
	}
	// for loop while we haven't reached 5 nodes
	for ; max_nodes > 0; max_nodes = max_nodes - 1 {
		// listen for new connection requests
		conn, err := listen.Accept()

		if err != nil {
			// not breaking error, that is a node side error
			fmt.Println("An error occured trying to accept new connection.", err)
		}

		// node handler
		if _, ok := conn.RemoteAddr().(*net.TCPAddr); ok {

			// let's make the last node the leader (even if they aren't the closest)
			if max_nodes == 1 {
				leader = true
			}

			check := false
			// assign random ID
			// between 8002 and 8999
			var ID int
			for check == false {

				ID = 8000 + rand.Intn(997) + 2
				// sanity check to see if someone has that ID
				ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(ID))
				if err != nil {
					continue
				}
				check = true
				_ = ln.Close()
			}
			nodeList[conn] = Node{ID: ID, Leader: false, Port: ID, Conn: conn}
			info <- "Node: " + strconv.Itoa(ID) + " has joined"
			// send the Node info to the node in question
			io.WriteString(conn, strconv.Itoa(ID) + ":" + strconv.Itoa(ID) + ":" + strconv.FormatBool(leader) + "\n")
			// monitor the new connection in case it drops
			go connectionCheck(conn, info, ID)
		}

	}

	// at this point we have gotten all 5 nodes on board
	// we now need to send an update to them
	for conn, _ := range nodeList {
		go sendInfo(conn)
	}

	var empty int
	fmt.Scan(&empty)
}