package ot

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"golang.org/x/crypto/blake2b"
)

const (
	BCAST_ID = 0
)

type MessengerClient struct {
	tx      []*Message
	rx      map[string]any
	http    *http.Client
	baseURL string

	SessionId string
}

// Connect builds an HTTP client targeting the messenger server and waits until
// the server is reachable (mirrors the Rust client's connect-with-retries).
//
// `hostport` may be a bare "host:port" or a full "http(s)://host:port" URL.
func (cl *MessengerClient) Connect(hostport string) (*MessengerClient, error) {
	base := hostport
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	base = strings.TrimRight(base, "/")

	httpClient := &http.Client{}

	// Wait for the server to become reachable.
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		resp, err := httpClient.Get(base + "/ping")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			lastErr = nil
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		return nil, errors.Wrapf(lastErr, "[MessengerClient] failed to connect to server: %s", base)
	}

	if cl != nil {
		*cl = MessengerClient{
			tx:      make([]*Message, 0),
			rx:      make(map[string]any),
			http:    httpClient,
			baseURL: base,
		}
	}
	return cl, nil
}

func (cl *MessengerClient) Close() error {
	cl.http.CloseIdleConnections()
	return nil
}

// rpc POSTs a gob-encoded request body to `path` and decodes the gob response
// into `respOut` (which may be nil). It returns an error on non-200 status.
func (cl *MessengerClient) rpc(path string, reqBody any, respOut any) error {
	var buf bytes.Buffer
	if reqBody != nil {
		if err := gob.NewEncoder(&buf).Encode(reqBody); err != nil {
			return errors.Wrapf(err, "[MessengerClient] failed to encode request to %s", path)
		}
	}

	resp, err := cl.http.Post(cl.baseURL+path, "application/octet-stream", &buf)
	if err != nil {
		return errors.Wrapf(err, "[MessengerClient] request to %s failed", path)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.Newf("[MessengerClient] %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	if respOut != nil {
		if err := gob.NewDecoder(resp.Body).Decode(respOut); err != nil {
			return errors.Wrapf(err, "[MessengerClient] failed to decode response from %s", path)
		}
	}
	return nil
}

func (cl *MessengerClient) NewSession(cfgReq *SessionConfig) (string, error) {
	var sid SessionId
	if err := cl.rpc("/new_session", cfgReq, &sid); err != nil {
		return "", err
	}
	cl.SessionId = sid.Value
	return sid.Value, nil
}

func (cl *MessengerClient) NewSessionEasy() (string, error) {
	return cl.NewSession(&SessionConfig{})
}

func (cl *MessengerClient) GetSessionConfig(sessionId string) (*SessionConfig, error) {
	var cfg SessionConfig
	if err := cl.rpc("/get_session_config", &SessionId{Value: sessionId}, &cfg); err != nil {
		return nil, err
	}
	cl.SessionId = cfg.SessionId
	return &cfg, nil
}

func (cl *MessengerClient) Ping() (string, error) {
	resp, err := cl.http.Get(cl.baseURL + "/ping")
	if err != nil {
		return "", errors.Wrapf(err, "[MessengerClient] failed to ping server")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.Newf("[MessengerClient] ping returned %d", resp.StatusCode)
	}
	var echo EchoMessage
	if err := gob.NewDecoder(resp.Body).Decode(&echo); err != nil {
		return "", errors.Wrapf(err, "[MessengerClient] failed to decode ping response")
	}
	return echo.Value, nil
}

func (cl *MessengerClient) DirectSend(
	obj any,
	sid string,
	topic string,
	src int,
	dst int,
	seq int,
) error {
	buf0 := new(bytes.Buffer)
	err := gob.NewEncoder(buf0).Encode(obj)
	if err != nil {
		return errors.Wrapf(err, "[DirectSend] failed to serialize object: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
	}
	msg := &Message{
		Sid: sid, Topic: topic, Src: uint64(src), Dst: uint64(dst), Seq: uint64(seq),
		Val: buf0.Bytes(),
	}
	req := &VecMessage{Values: []*Message{msg}}

	if err := cl.rpc("/inbox", req, &Void{}); err != nil {
		return errors.Wrapf(err, "[ DirectSend ] failed to post object: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
	}

	if os.Getenv("GARBLED_VERBOSE") != "" {
		log.Printf(
			"finish DirectSend. sid=[%s], topic=[%s], src=%d, dst=%d, seq=%d, size=%dbytes.",
			sid, topic, src, dst, seq, len(msg.Val),
		)
	}
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
	reqMsg := &Message{
		Sid: sid, Topic: topic, Src: uint64(src), Dst: uint64(dst), Seq: uint64(seq),
		Val: nil,
	}
	req := &VecMessage{Values: []*Message{reqMsg}}

	var resp VecMessage
	if err := cl.rpc("/outbox", req, &resp); err != nil {
		return errors.Wrapf(err, "[ DirectRecv ] failed to fetch object: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
	}
	if len(resp.Values) != 1 {
		return errors.Newf("[ DirectRecv ] received bad response: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
	}
	data := resp.Values[0].Val
	nbytes := len(data)

	buf := bytes.NewBuffer(data)
	if err := gob.NewDecoder(buf).Decode(out); err != nil {
		return errors.Wrapf(err, "[ DirectRecv ] failed to deserialize object: "+
			"query = (%s, %s, %d, %d, %d)", sid, topic, src, dst, seq)
	}
	if os.Getenv("GARBLED_VERBOSE") != "" {
		log.Printf(
			"finish DirectRecv. sid=[%s], topic=[%s], src=%d, dst=%d, seq=%d, size=%dbytes.",
			sid, topic, src, dst, seq, nbytes,
		)
	}

	return nil
}

func (cl *MessengerClient) MpcClear() {
	cl.tx = make([]*Message, 0)
	cl.rx = make(map[string]any)
}

// PrimaryKey derives the message cache key, mirroring the Rust crate's
// `primary_key`: blake2b with 16-byte output over
// sid || topic || src_le || dst_le || seq_le, hex-encoded.
func PrimaryKey(sid string, topic string, src uint64, dst uint64, seq uint64) string {
	ha, _ := blake2b.New(16, nil)
	ha.Write([]byte(sid))
	ha.Write([]byte(topic))
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], src)
	ha.Write(b[:])
	binary.LittleEndian.PutUint64(b[:], dst)
	ha.Write(b[:])
	binary.LittleEndian.PutUint64(b[:], seq)
	ha.Write(b[:])
	return hex.EncodeToString(ha.Sum(nil))
}
