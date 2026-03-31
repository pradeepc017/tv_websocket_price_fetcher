package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
	"runtime"

	"github.com/gorilla/websocket"
)

var debug bool

// --- symbols ---
var symbols = []string{
	"OANDA:XAUUSD",
	"OANDA:XAGUSD",
	"OANDA:XPTUSD",
	"OANDA:XPDUSD",
}

// --- quote (no pointers = low GC) ---
type Quote struct {
	Price     float64 `json:"price"`
	Bid       float64 `json:"bid"`
	Ask       float64 `json:"ask"`
	Volume    float64 `json:"volume"`
	Timestamp int64   `json:"timestamp"`

	HasPrice bool
	HasBid   bool
	HasAsk   bool
}

// --- global store ---
var (
	data = make(map[string]*Quote, 4)
	mu   sync.RWMutex
)

// --- websocket ---
const wsURL = "wss://data.tradingview.com/socket.io/websocket"

// --- helpers ---
func logDebug(v ...interface{}) {
	if debug {
		log.Println(v...)
	}
}

func genSession() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 12)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "qs_" + string(b)
}

func wrap(msg string) string {
	return fmt.Sprintf("~m~%d~m~%s", len(msg), msg)
}

func send(ws *websocket.Conn, method string, params interface{}) {
	payload := map[string]interface{}{
		"m": method,
		"p": params,
	}
	b, _ := json.Marshal(payload)
	msg := wrap(string(b))

	logDebug("SEND:", msg)

	ws.WriteMessage(websocket.TextMessage, []byte(msg))
}

// --- FAST parser (no heavy ops) ---
func parseMessages(raw string) []string {
	out := make([]string, 0, 4)

	for {
		if !strings.HasPrefix(raw, "~m~") {
			break
		}

		raw = raw[3:]
		i := strings.Index(raw, "~m~")
		if i < 0 {
			break
		}

		var length int
		fmt.Sscanf(raw[:i], "%d", &length)

		raw = raw[i+3:]
		if len(raw) < length {
			break
		}

		out = append(out, raw[:length])
		raw = raw[length:]
	}

	return out
}

// --- message handler ---
func handleMessage(msg string) {
	// 🔥 skip everything except qsd
	if !strings.Contains(msg, `"m":"qsd"`) {
		return
	}

	logDebug("RECV:", msg)

	var m struct {
		M string            `json:"m"`
		P []json.RawMessage `json:"p"`
	}

	if err := json.Unmarshal([]byte(msg), &m); err != nil {
		return
	}

	if m.M != "qsd" || len(m.P) < 2 {
		return
	}

	var payload struct {
		N string             `json:"n"`
		V map[string]float64 `json:"v"`
	}

	if err := json.Unmarshal(m.P[1], &payload); err != nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	q, ok := data[payload.N]
	if !ok {
		q = &Quote{}
		data[payload.N] = q
	}

	// ⏱ throttle: 1 update/sec
	now := time.Now().Unix()
	if q.Timestamp == now {
		return
	}

	if v, ok := payload.V["lp"]; ok {
		q.Price = v
		q.HasPrice = true
	}
	if v, ok := payload.V["bid"]; ok {
		q.Bid = v
		q.HasBid = true
	}
	if v, ok := payload.V["ask"]; ok {
		q.Ask = v
		q.HasAsk = true
	}
	if v, ok := payload.V["volume"]; ok {
		q.Volume = v
	}

	q.Timestamp = now
}

// --- websocket loop ---
func connect() {
	backoff := time.Second

	for {
		logDebug("Connecting...")

		ws, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
			"Origin":     []string{"https://www.tradingview.com"},
			"User-Agent": []string{"Mozilla/5.0"},
		})

		if err != nil {
			logDebug("Dial error:", err)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}

		logDebug("Connected")
		backoff = time.Second

		session := genSession()

		send(ws, "quote_create_session", []interface{}{session})

		send(ws, "quote_set_fields", []interface{}{
			session, "lp", "bid", "ask", "volume",
		})

		for _, sym := range symbols {
			send(ws, "quote_add_symbols", []interface{}{session, sym})
		}

		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				logDebug("Read error:", err)
				ws.Close()
				break
			}

			rawMsgs := parseMessages(string(msg))
			for _, m := range rawMsgs {
				handleMessage(m)
			}
		}
	}
}

// --- API ---
func latestHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	out := make(map[string]*Quote, len(data))

	for sym, q := range data {
		if !(q.HasPrice && q.HasBid && q.HasAsk) {
			continue
		}
		out[sym] = q
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func main() {
	flag.BoolVar(&debug, "debug", false, "enable debug logs")
	flag.Parse()
	runtime.GOMAXPROCS(1)

	rand.Seed(time.Now().UnixNano())

	go connect()

	http.HandleFunc("/latest", latestHandler)

	log.Println("Server running on :8080")
	log.Println("Debug mode:", debug)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
