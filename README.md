a P2P networking layer built on TCP using Go

To create the cluster network:

Initialize a central server to add new nodes:
go run initializer.go

To add a new cluster to the network:
go run node.go

Do - go run node.go in 5 separate tabs

The last created node will become the base leader, to test the consensus, end the last tab's connection.