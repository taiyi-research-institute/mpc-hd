package ot

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/markkurossi/mpc/pb"
	"golang.org/x/crypto/blake2b"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	BCAST_ID = 0
)

type MessengerClient struct {
	tx   []*pb.Message
	rx   map[string]any
	conn *grpc.ClientConn

	SessionId string
}

func (cl *MessengerClient) Connect(hostport string) (*MessengerClient, error) {
	conn, err := grpc.NewClient(
		fmt.Sprintf("%s", hostport),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1048576*32),
		),
	)
	if err != nil {
		return nil, err
	}
	if cl != nil {
		*cl = MessengerClient{
			tx:   make([]*pb.Message, 0),
			rx:   make(map[string]any),
			conn: conn,
		}
	}
	return cl, nil
}

func (cl *MessengerClient) Close() error {
	return cl.conn.Close()
}

func (cl *MessengerClient) stub() pb.MpcSessionManagerClient {
	return pb.NewMpcSessionManagerClient(cl.conn)
}

func (cl *MessengerClient) GrpcNewSession(
	cfg_req *pb.SessionConfig,
) (string, error) {
	// ceremony
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	cfg_resp, err := stub.NewSession(ctx, cfg_req)
	if err != nil {
		return "", err
	}
	cl.SessionId = cfg_resp.Value
	return cfg_resp.Value, nil
}

func (cl *MessengerClient) GrpcNewSessionEasy() (string, error) {
	// ceremony
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	resp, err := stub.NewSession(ctx, &pb.SessionConfig{})
	if err != nil {
		return "", err
	}
	cl.SessionId = resp.Value
	return resp.Value, nil
}

func (cl *MessengerClient) GrpcGetSessionConfig(
	session_id string,
) (*pb.SessionConfig, error) {
	// ceremony
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	cfg, err := stub.GetSessionConfig(ctx, &pb.SessionId{Value: session_id})
	if err != nil {
		return nil, err
	}
	cl.SessionId = cfg.SessionId
	return cfg, nil
}

func (cl *MessengerClient) DirectSend(
	obj any,
	sid string,
	topic string,
	src int,
	dst int,
	seq int,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	buf0 := new(bytes.Buffer)
	err := gob.NewEncoder(buf0).Encode(obj)
	if err != nil {
		err = errors.Wrapf(err, "[DirectSend] failed to serialize object: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
		return err
	}
	req0 := &pb.Message{
		Sid: sid, Topic: topic, Src: uint64(src), Dst: uint64(dst), Seq: uint64(seq),
		Val: buf0.Bytes(),
	}
	req := &pb.VecMessage{Values: []*pb.Message{req0}}

	if _, err = stub.Inbox(ctx, req); err != nil {
		err = errors.Wrapf(err, "[ DirectSend ] failed to post object: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
		return err
	}

	log.Printf(
		"finish DirectSend. sid=[%s], topic=[%s], src=%d, dst=%d, seq=%d, size=%dbytes.",
		sid, topic, src, dst, seq, len(req0.Val),
	)
	return nil
}

func (cl *MessengerClient) DirectRecv(
	out any,
	sid string,
	topic string,
	src int,
	dst int,
	seq int,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	req0 := &pb.Message{
		Sid: sid, Topic: topic, Src: uint64(src), Dst: uint64(dst), Seq: uint64(seq),
		Val: nil,
	}
	req := &pb.VecMessage{Values: []*pb.Message{req0}}

	resp0, err := stub.Outbox(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "[ DirectRecv ] failed to post object: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
		return err
	}
	if len(resp0.Values) != 1 {
		err = errors.Wrapf(err, "[ DirectRecv ] received bad response: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
		return err
	}
	resp := resp0.Values[0].Val
	nbytes := len(resp)

	buf := bytes.NewBuffer(resp)
	err = gob.NewDecoder(buf).Decode(out)
	if err != nil {
		err = errors.Wrapf(err, "[ DirectRecv ] failed to deserialize object: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
		return err
	}
	log.Printf(
		"finish DirectRecv. sid=[%s], topic=[%s], src=%d, dst=%d, seq=%d, size=%dbytes.",
		sid, topic, src, dst, seq, nbytes,
	)

	return nil
}

func (cl *MessengerClient) MpcClear() {
	cl.tx = make([]*pb.Message, 0)
	cl.rx = make(map[string]any)
}

func PrimaryKey(args ...any) string {
	ha, _ := blake2b.New256(nil)
	for _, arg := range args {
		buf := bytes.NewBuffer(nil)
		err := gob.NewEncoder(buf).Encode(arg)
		if err != nil {
			panic(err)
		}
		ha.Write(buf.Bytes())
	}
	return hex.EncodeToString(ha.Sum(nil))
}

// Thanks to
// https://github.com/grpc/grpc-go/blob/master/examples/route_guide/client/client.go
