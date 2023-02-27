// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package v4wire implements the Discovery v4 Wire Protocol.
package v4wire

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/DogeProtocol/dp/crypto/cryptobase"
	"github.com/DogeProtocol/dp/crypto/signaturealgorithm"

	"net"
	"time"

	"github.com/DogeProtocol/dp/crypto"
	"github.com/DogeProtocol/dp/p2p/enode"
	"github.com/DogeProtocol/dp/p2p/enr"
	"github.com/DogeProtocol/dp/rlp"
)

// RPC packet types
const (
	PingPacket = iota + 1 // zero is 'reserved'
	PongPacket
	FindnodePacket
	NeighborsPacket
	ENRRequestPacket
	ENRResponsePacket
)

// RPC request structures
type (
	Ping struct {
		Version    uint
		From, To   Endpoint
		Expiration uint64
		ENRSeq     uint64 `rlp:"optional"` // Sequence number of local record, added by EIP-868.

		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// Pong is the reply to ping.
	Pong struct {
		// This field should mirror the UDP envelope address
		// of the ping packet, which provides a way to discover the
		// the external address (after NAT).
		To         Endpoint
		ReplyTok   []byte // This contains the hash of the ping packet.
		Expiration uint64 // Absolute timestamp at which the packet becomes invalid.
		ENRSeq     uint64 `rlp:"optional"` // Sequence number of local record, added by EIP-868.

		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// Findnode is a query for nodes close to the given target.
	Findnode struct {
		Target     Pubkey
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// Neighbors is the reply to findnode.
	Neighbors struct {
		Nodes      []Node
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// enrRequest queries for the remote node's record.
	ENRRequest struct {
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// enrResponse is the reply to enrRequest.
	ENRResponse struct {
		ReplyTok []byte // Hash of the enrRequest packet.
		Record   enr.Record
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}
)

// This number is the maximum number of neighbor nodes in a Neigbors packet.
const MaxNeighbors = 1

// This code computes the MaxNeighbors constant value.

// func init() {
// 	var maxNeighbors int
// 	p := Neighbors{Expiration: ^uint64(0)}
// 	maxSizeNode := Node{IP: make(net.IP, 16), UDP: ^uint16(0), TCP: ^uint16(0)}
// 	for n := 0; ; n++ {
// 		p.Nodes = append(p.Nodes, maxSizeNode)
// 		size, _, err := rlp.EncodeToReader(p)
// 		if err != nil {
// 			// If this ever happens, it will be caught by the unit tests.
// 			panic("cannot encode: " + err.Error())
// 		}
// 		if headSize+size+1 >= 1280 {
// 			maxNeighbors = n
// 			break
// 		}
// 	}
// 	//fmt.println("maxNeighbors", maxNeighbors)
// }

// Pubkey represents an encoded 64-byte secp256k1 public key.

type Pubkey struct {
	PubBytes []byte
}

func WirePubKeyEquals(pub1 Pubkey, pub2 Pubkey) bool {
	if bytes.Compare(pub1.PubBytes, pub2.PubBytes) == 0 {
		return true
	}

	return false
}

func CreateWirePubKey(pubBytes []byte) Pubkey {
	return Pubkey{PubBytes: pubBytes}
}

func CreateWirePubKeyFromSigAlg(pubKey *signaturealgorithm.PublicKey) Pubkey {
	pubBytes, err := cryptobase.SigAlg.SerializePublicKey(pubKey)
	if err != nil {
		panic(err)
	}
	return Pubkey{PubBytes: pubBytes}
}

// ID returns the node ID corresponding to the public key.
func (e Pubkey) ID() enode.ID {
	return enode.ID(crypto.Keccak256Hash(e.PubBytes))
}

// Node represents information about a node.
type Node struct {
	IP  net.IP // len 4 for IPv4 or 16 for IPv6
	UDP uint16 // for discovery protocol
	TCP uint16 // for RLPx protocol
	ID  Pubkey
}

// Endpoint represents a network endpoint.
type Endpoint struct {
	IP  net.IP // len 4 for IPv4 or 16 for IPv6
	UDP uint16 // for discovery protocol
	TCP uint16 // for RLPx protocol
}

// NewEndpoint creates an endpoint.
func NewEndpoint(addr *net.UDPAddr, tcpPort uint16) Endpoint {
	ip := net.IP{}
	if ip4 := addr.IP.To4(); ip4 != nil {
		ip = ip4
	} else if ip6 := addr.IP.To16(); ip6 != nil {
		ip = ip6
	}
	return Endpoint{IP: ip, UDP: uint16(addr.Port), TCP: tcpPort}
}

type Packet interface {
	// packet name and type for logging purposes.
	Name() string
	Kind() byte
}

func (req *Ping) Name() string { return "PING/v4" }
func (req *Ping) Kind() byte   { return PingPacket }

func (req *Pong) Name() string { return "PONG/v4" }
func (req *Pong) Kind() byte   { return PongPacket }

func (req *Findnode) Name() string { return "FINDNODE/v4" }
func (req *Findnode) Kind() byte   { return FindnodePacket }

func (req *Neighbors) Name() string { return "NEIGHBORS/v4" }
func (req *Neighbors) Kind() byte   { return NeighborsPacket }

func (req *ENRRequest) Name() string { return "ENRREQUEST/v4" }
func (req *ENRRequest) Kind() byte   { return ENRRequestPacket }

func (req *ENRResponse) Name() string { return "ENRRESPONSE/v4" }
func (req *ENRResponse) Kind() byte   { return ENRResponsePacket }

// Expired checks whether the given UNIX time stamp is in the past.
func Expired(ts uint64) bool {
	return time.Unix(int64(ts), 0).Before(time.Now())
}

// Encoder/decoder.

const (
	macSize = 32
)

var (
	ErrPacketTooSmall = errors.New("too small")
	ErrBadHash        = errors.New("bad hash")
	ErrBadPoint       = errors.New("invalid curve point")
)

// Decode reads a discovery v4 packet.
func Decode(input []byte) (Packet, Pubkey, []byte, error) {

	inputSize := len(input)
	if inputSize < macSize {

		return nil, Pubkey{}, nil, ErrPacketTooSmall
	}

	sigSize := cryptobase.SigAlg.SignatureWithPublicKeyLength() //todo: make this dynamic
	if sigSize > inputSize {
		return nil, Pubkey{}, nil, ErrPacketTooSmall
	}

	headSize := inputSize - sigSize - macSize
	if headSize < 0 || headSize > inputSize {

		return nil, Pubkey{}, nil, ErrPacketTooSmall
	}

	if inputSize < (sigSize + macSize + headSize) {

		return nil, Pubkey{}, nil, ErrPacketTooSmall
	}

	hash := input[:macSize]
	sig := input[macSize : inputSize-headSize]
	sigdata := input[inputSize-headSize:]

	//hash, sig, sigdata := input[:macSize], input[macSize:inputSize-headSize], input[inputSize-headSize:]
	shouldhash := crypto.Keccak256(input[macSize:])
	if !bytes.Equal(hash, shouldhash) {

		return nil, Pubkey{}, nil, ErrBadHash
	}

	digest := crypto.Keccak256(input[inputSize-headSize:])

	fromKey, err := recoverNodeKey(digest, sig)
	if err != nil {
		return nil, fromKey, hash, err
	}

	var req Packet

	switch ptype := sigdata[0]; ptype {
	case PingPacket:
		req = new(Ping)
	case PongPacket:
		req = new(Pong)
	case FindnodePacket:
		req = new(Findnode)
	case NeighborsPacket:
		req = new(Neighbors)
	case ENRRequestPacket:
		req = new(ENRRequest)
	case ENRResponsePacket:
		req = new(ENRResponse)
	default:
		return nil, fromKey, hash, fmt.Errorf("unknown type: %d", ptype)
	}
	s := rlp.NewStream(bytes.NewReader(sigdata[1:]), 0)

	err = s.Decode(req)

	return req, fromKey, hash, err
}

// Encode encodes a discovery packet.
func Encode(priv *signaturealgorithm.PrivateKey, req Packet) (packet, hash []byte, err error) {
	b := new(bytes.Buffer)

	b.WriteByte(req.Kind())
	if err := rlp.Encode(b, req); err != nil {

		return nil, nil, err
	}
	packetreq := b.Bytes()

	digest := crypto.Keccak256(packetreq[:])

	sig, err := cryptobase.SigAlg.Sign(digest, priv)
	if err != nil {
		return nil, nil, err
	}

	// Add the hash to the front. Note: this doesn't protect the packet in any way.

	hash = crypto.Keccak256(append(sig, packetreq...))

	packet = append(packet, append(hash, append(sig, packetreq...)...)...)

	return packet, hash, nil
}

// recoverNodeKey computes the public key used to sign the given hash from the signature.
func recoverNodeKey(hash, sig []byte) (key Pubkey, err error) {
	pubkey, err := cryptobase.SigAlg.PublicKeyBytesFromSignature(hash, sig)
	if err != nil {
		return key, err
	}
	key.PubBytes = make([]byte, len(pubkey))
	copy(key.PubBytes, pubkey[:])
	return key, nil
}

// EncodePubkey encodes a secp256k1 public key.
func EncodePubkey(key *signaturealgorithm.PublicKey) Pubkey {
	var e Pubkey
	pubBytes, err := cryptobase.SigAlg.SerializePublicKey(key)
	if err != nil {
		return e
	}
	e.PubBytes = make([]byte, len(pubBytes))
	copy(e.PubBytes, pubBytes[:])
	return e
}

// DecodePubkey reads an encoded secp256k1 public key.

func DecodePubkey(e Pubkey) (*signaturealgorithm.PublicKey, error) {

	keyBytes := e.PubBytes //todo: fix
	count := 0
	for _, v := range keyBytes {
		if v == 0 {
			count = count + 1
		}
	}
	if count == len(keyBytes) {
		return nil, errors.New("all zero public key")
	}

	return cryptobase.SigAlg.DecodePublicKey(keyBytes)
}
