// Copyright 2015 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

// bootnode runs a bootstrap node for the Ethereum Discovery Protocol.
package main

import (
	"flag"
	"fmt"
	"github.com/DogeProtocol/dp/crypto/cryptobase"
	"github.com/DogeProtocol/dp/crypto/signaturealgorithm"
	"net"
	"os"

	"github.com/DogeProtocol/dp/cmd/utils"
	"github.com/DogeProtocol/dp/log"
	"github.com/DogeProtocol/dp/p2p/discover"
	"github.com/DogeProtocol/dp/p2p/enode"
	"github.com/DogeProtocol/dp/p2p/nat"
	"github.com/DogeProtocol/dp/p2p/netutil"
)

func main() {
	var (
		listenAddr  = flag.String("addr", ":30301", "listen address")
		genKey      = flag.String("genkey", "", "generate a node key")
		writeAddr   = flag.Bool("writeaddress", false, "write out the node's public key and quit")
		nodeKeyFile = flag.String("nodekey", "", "private key filename")
		nodeKeyHex  = flag.String("nodekeyhex", "", "private key as hex (for testing)")
		natdesc     = flag.String("nat", "none", "port mapping mechanism (any|none|upnp|pmp|extip:<IP>)")
		netrestrict = flag.String("netrestrict", "", "restrict network communication to the given IP networks (CIDR masks)")
		runv5       = flag.Bool("v5", false, "run a v5 topic discovery bootnode")
		verbosity   = flag.Int("verbosity", int(log.LvlInfo), "log verbosity (0-5)")
		vmodule     = flag.String("vmodule", "", "log verbosity pattern")

		nodeKey *signaturealgorithm.PrivateKey
		err     error
	)
	flag.Parse()

	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(false)))
	glogger.Verbosity(log.Lvl(*verbosity))
	glogger.Vmodule(*vmodule)
	log.Root().SetHandler(glogger)

	natm, err := nat.Parse(*natdesc)
	if err != nil {
		utils.Fatalf("-nat: %v", err)
	}
	switch {
	case *genKey != "":
		nodeKey, err = cryptobase.SigAlg.GenerateKey()
		if err != nil {
			utils.Fatalf("could not generate key: %v", err)
		}
		if err = cryptobase.SigAlg.SavePrivateKeyToFile(*genKey, nodeKey); err != nil {
			utils.Fatalf("%v", err)
		}
		if !*writeAddr {
			return
		}
	case *nodeKeyFile == "" && *nodeKeyHex == "":
		utils.Fatalf("Use -nodekey or -nodekeyhex to specify a private key")
	case *nodeKeyFile != "" && *nodeKeyHex != "":
		utils.Fatalf("Options -nodekey and -nodekeyhex are mutually exclusive")
	case *nodeKeyFile != "":
		if nodeKey, err = cryptobase.SigAlg.LoadPrivateKeyFromFile(*nodeKeyFile); err != nil {
			utils.Fatalf("-nodekey: %v", err)
		}
	case *nodeKeyHex != "":
		if nodeKey, err = cryptobase.SigAlg.HexToPrivateKey(*nodeKeyHex); err != nil {
			utils.Fatalf("-nodekeyhex: %v", err)
		}
	}

	if *writeAddr {
		_, err := cryptobase.SigAlg.SerializePublicKey(&nodeKey.PublicKey)
		if err != nil {
			panic(err)
		}
		os.Exit(0)
	}

	var restrictList *netutil.Netlist
	if *netrestrict != "" {
		restrictList, err = netutil.ParseNetlist(*netrestrict)
		if err != nil {
			utils.Fatalf("-netrestrict: %v", err)
		}
	}

	sessionManager, err := discover.CreateDpUdpSessionManager(*listenAddr)
	if err != nil {
		panic(err)
	}

	realaddr := sessionManager.LocalAddr().(*net.UDPAddr)
	if natm != nil {
		if !realaddr.IP.IsLoopback() {
			go nat.Map(natm, nil, "udp", realaddr.Port, realaddr.Port, "ethereum discovery")
		}
		if ext, err := natm.ExternalIP(); err == nil {
			realaddr = &net.UDPAddr{IP: ext, Port: realaddr.Port}
		}
	}

	printNotice(&nodeKey.PublicKey, *realaddr)

	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, nodeKey)
	cfg := discover.Config{
		PrivateKey:  nodeKey,
		NetRestrict: restrictList,
	}
	if *runv5 {
		if _, err := discover.ListenV5(nil, ln, cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	} else {
		if _, err := discover.ListenUDP(sessionManager, ln, cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	}

	select {}
}

func printNotice(nodeKey *signaturealgorithm.PublicKey, addr net.UDPAddr) {
	if addr.IP.IsUnspecified() {
		addr.IP = net.IP{127, 0, 0, 1}
	}
	n := enode.NewV4(nodeKey, addr.IP, 0, addr.Port)
	fmt.Println(n.URLv4())
	fmt.Println("Note: you're using cmd/bootnode, a developer tool.")
	fmt.Println("We recommend using a regular node as bootstrap node for production deployments.")
}
