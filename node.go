package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

type Node struct {
	ID         int
	Port       int
	Leader     bool
	ServerConn net.Conn
}

var (
	myNode = Node{}
	nodeList = make(map[int]*Node)
	serverConn = make(map[int]net.Conn)
	leaderID int
	leader bool
	errorMsg = " "
)

// concurrent printer for each node when it gets new server info
func printServerInfo(serverInfo chan string) {
	for {
		select {
		case info := <-serverInfo:
			{
				fmt.Println(info)
			}
		}
	}
}

// nice check of all the connected nodes
func printConnected() {
	println("--Connected Processes -- ")
	for ID, _ := range nodeList {

		fmt.Printf("\tID: %d\n", ID)
	}

}

// the core of our "algo". First if is whether we are the new leader
// Else is someone else, which means we need to alert all the ones that 
// are closer they need to participate. This is important since if the ones
// closer never respond, then we can assume we are the closest (and in turn making us the leader)
func newElection() {

	if areWeLeader() {
		// let everyone know we're the leader
		for id, _ := range nodeList {
			leader = true
			errorMsg = " "
			io.WriteString(serverConn[id], "l" + strconv.Itoa(myNode.ID) + "\n")
		}
		fmt.Println("I am the new leader")

	} else {
		for id, _ := range nodeList {
			if myNode.ID < id {
				sendCount += 1
				io.WriteString(serverConn[id], "e" + strconv.Itoa(myNode.ID) + "\n")
			}
		}
	}
	// let's wait some time before we think nobody has responded
	time.Sleep(time.Duration(5) * time.Microsecond * 100000)
	// no ok's from closer nodes
	if okCount == 0 {
		for id, _ := range nodeList {
			leader = true
			errorMsg = " "
			io.WriteString(serverConn[id], "l" + strconv.Itoa(myNode.ID) + "\n")

		}
		fmt.Println("I am the new leader")

	}
}

func areWeLeader() bool {
	return myNode.ID == getleaderId()
}

// Our algorithm! Reaching consensus by choosing closest to mid
func getleaderId() int {

	max := myNode.ID
	for id, _ := range nodeList {
		if id > max {
			max = id
		}
	}
	return max
}

// Handling server connections, specifically for leader info. 
// Each one runs a conccurent instance of serverCheck
func connectToNodes() {
	for id, node := range nodeList {
		con, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(node.Port))
		if err != nil {
			log.Fatal(err)
		}
		nodeList[id].ServerConn = con

		go serverCheck(nodeList[id].ServerConn)

	}
}

// our listener. Using this, we will accept any and all new info from nodes,
// as well as open up connections to listen for drops in connections
func connector(nodeInfo chan string, removeInfo chan int, baseInfo chan string, ch chan bool) {
	_ = <-ch
	fmt.Println("my node ID: ", myNode.ID)

	fmt.Println("Listening at port: ", strconv.Itoa(myNode.Port))

	listen, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(myNode.Port))
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := listen.Accept()

		if err != nil {
			fmt.Println("Error while accepting a new connection", err)
			
		}

		go handleConnection(conn, nodeInfo, removeInfo, baseInfo)

	}
}

// This is what starts whenever 5 nodes join the network. If we get an ok from the initiator, it's go time
// we'll leave it open afterwards in case other nodes join
func readFromInitiator(conn net.Conn, ch chan<- string, blockCh chan bool, connectCh chan bool) {
	reader := bufio.NewReader(conn)

	// open for loop while we wait for initiation
	for {

		message, err := reader.ReadString('\n')
		// the only other message we'll get is a new node joining the network
		if message != "OK\n" {
			if err != nil {
				break
			}
			s := strings.Split(message, ":")
			id, _ := strconv.Atoi(s[0])
			port, _ := strconv.Atoi(s[1])
			leader, _ := strconv.ParseBool(s[2][:len(s[2])-1])
			if leader == true {
				leaderID = id
				fmt.Println("leader id: ", leaderID)

			}
			// if myNode is empty, let's get the info
			if (Node{}) == myNode {
				myNode.Port = port
				myNode.ID = id
				myNode.Leader = leader
				// block someone else from using this channel
				blockCh <- true

			} else {
				nodeList[id] = &Node{Port: port, Leader: leader, ID: id}
			}

		} else {
			fmt.Println("OK")
			connectToNodes()
		}
	}
}

// funnel of all updates that occur between node connections (conn drops/ conn adds/ conn relays)
func nodeChecker(nodeInfo chan string, removeInfo chan int, baseInfo chan string) {

	for {
		select {
			// a new node has been introduced to the system
		case node := <-nodeInfo:
			{
				fmt.Printf("ID %s is now connected\n", node)

			}
			// we need to relay all the nodes whenever one has been dropped so the others can be aware
		case node := <-baseInfo:
			{

				for _, con := range serverConn {

					io.WriteString(con, node + "\n")
				}
			}
			// we found out that one node has been dropped
		case node := <-removeInfo:
			delete(nodeList, node)
			delete(serverConn, node)
			printConnected()
		}

	}
}

var okCount = 0
var sendCount = 0
// operations between server networks. Basically relay of leader/coordinator messages between the nodes
// messages have 4 types of "OP_CODES"
// d : something occured when trying to get response from a node (someone most likely disconnected)
// l : A new leader has been elected
// e : A new election process has been started, this means we need to initiate our own internal election
// o : A node accepts the leader request as the best for the job
func serverCheck(conn net.Conn) {

	// send to the particular connection our ID and see what their response is
	io.WriteString(conn, strconv.Itoa(myNode.ID)+"\n")
	// response reader
	reader := bufio.NewReader(conn)
	// open for loop while we wait for response
	for {
		msg, err := reader.ReadString('\n')
		if len(msg) > 0 {
			fmt.Println()
			if string(msg[0]) == "d" {
				fmt.Printf("Node: " + msg[1:len(msg)-1] + " detected an error within the network.\n")
				errorMsg = "ERR"
			} else if string(msg[0]) == "l" {
				// sender is leader
				fmt.Printf("New leader ID: " + msg[1:len(msg)-1] + "\n")
				leader = true
				// need to reset error since new leader election usually means prev leader disconnected
				errorMsg = " "

			} else if string(msg[0]) == "e" {
				fmt.Printf("Election request by node: " + msg[1:len(msg)-1] + "\n")
				id, _ := strconv.Atoi(msg[1 : len(msg)-1])
				// let sender know we're fine and able to participate in election
				io.WriteString(serverConn[id], "GOOD"+"\n")
				newElection()
			} else if string(msg[0]) == "o" {
				fmt.Printf("ID: " + msg[1:len(msg)-1] + " STATUS - GOOD" + "\n")
				okCount += 1
			}

		}
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

// this is how we check whether someone has disconnected or not, and the steps that we need to take after we detect it
// especially important if disconnection wad from leader
// open communication for loop running with all nodes (defers if we get err)
func handleConnection(conn net.Conn, nodeInfo chan string, removeInfo chan int, baseInfo chan string) {

	// open up reader 
	reader := bufio.NewReader(conn)
	id, _ := reader.ReadString('\n')
	str_id := id[:len(id)-1]
	// set the node channel to the ID in question
	nodeInfo <- str_id
	ID, _ := strconv.Atoi(str_id)
	serverConn[ID] = conn

	// defered if for loop ends (ends if err != nil)
	defer func() {
		// very important if the ID that dropped is leader
		if ID == getleaderId() || ID == 8021 {

			removeInfo <- ID
			rand.Seed(time.Now().UnixNano())
			r := rand.Intn(6) + 1

			fmt.Printf("wait time: %d\n", r)
			time.Sleep(time.Duration(r) * time.Microsecond * 10000)
			// let's wait and see if other members have dropped off
			// or other information from our peers
			if errorMsg == " " {
				msg := "d" + strconv.Itoa(myNode.ID)

				// alert the other nodes of the disconnection
				for _, con := range serverConn {

					io.WriteString(con, msg+"\n")

				}
				leader = false
				go newElection()

			}
		} else {
			removeInfo <- ID
		}
	}()
	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			return
		}
	}
	defer conn.Close()
}

// main function getting everything started
// get all our channels and concurrent functions going
func main() {
	conn, err := net.Dial("tcp", ":8001")
	// someone needs to start the server
	if err != nil {
		fmt.Println("server not found")
		return
	}
	serverChan := make(chan string)

	blockCh := make(chan bool)
	connectCh := make(chan bool)
	nodeInfo := make(chan string)
	removeInfo := make(chan int)
	baseInfo := make(chan string)
	go readFromInitiator(conn, serverChan, blockCh, connectCh)
	go nodeChecker(nodeInfo, removeInfo, baseInfo)
	go connector(nodeInfo, removeInfo, baseInfo, blockCh)
	var input int
	fmt.Scanf("%d", &input)
}