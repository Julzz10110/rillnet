package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/pkg/config"
	"rillnet/tests/testutil"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireRedis(t *testing.T) *config.Config {
	t.Helper()
	if !testutil.RedisAvailable() {
		t.Skip("Redis not available (set RILLNET_REDIS_ADDRESS or start redis:7)")
	}
	return testutil.SmokeTestConfig()
}

func TestSmoke_ReadinessWithRedis(t *testing.T) {
	cfg := requireRedis(t)
	env := testutil.NewIngestTestEnv(t, cfg)
	defer env.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	env.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"dependencies":"ok"`)
}

func TestSmoke_AuthRegisterAndUnauthorized(t *testing.T) {
	cfg := requireRedis(t)
	env := testutil.NewIngestTestEnv(t, cfg)
	defer env.Close()

	body := `{"username":"smokeuser","email":"smoke@example.com","password":"password123"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	env.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var reg map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reg))
	token, ok := reg["access_token"].(string)
	require.True(t, ok && token != "")
	_, ok = reg["refresh_token"].(string)
	require.True(t, ok)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/streams", nil)
	env.Router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestSmoke_StreamLifecycleAPI(t *testing.T) {
	cfg := requireRedis(t)
	env := testutil.NewIngestTestEnv(t, cfg)
	defer env.Close()

	token := registerAndGetToken(t, env)
	streamID := createStream(t, env, token, "smoke-stream", "owner-peer-1")

	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/streams", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	env.Router.ServeHTTP(wList, reqList)
	require.Equal(t, http.StatusOK, wList.Code)

	wGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/streams/"+string(streamID), nil)
	reqGet.Header.Set("Authorization", "Bearer "+token)
	env.Router.ServeHTTP(wGet, reqGet)
	require.Equal(t, http.StatusOK, wGet.Code)

	joinBody := `{"peer_id":"subscriber-peer-1","is_publisher":false,"capabilities":{"max_bitrate":1000,"codecs":["vp8"]}}`
	wJoin := httptest.NewRecorder()
	reqJoin := httptest.NewRequest(http.MethodPost, "/api/v1/streams/"+string(streamID)+"/join", strings.NewReader(joinBody))
	reqJoin.Header.Set("Authorization", "Bearer "+token)
	reqJoin.Header.Set("Content-Type", "application/json")
	env.Router.ServeHTTP(wJoin, reqJoin)
	require.Equal(t, http.StatusOK, wJoin.Code, wJoin.Body.String())

	leaveBody := `{"peer_id":"subscriber-peer-1"}`
	wLeave := httptest.NewRecorder()
	reqLeave := httptest.NewRequest(http.MethodPost, "/api/v1/streams/"+string(streamID)+"/leave", strings.NewReader(leaveBody))
	reqLeave.Header.Set("Authorization", "Bearer "+token)
	reqLeave.Header.Set("Content-Type", "application/json")
	env.Router.ServeHTTP(wLeave, reqLeave)
	require.Equal(t, http.StatusOK, wLeave.Code, wLeave.Body.String())
}

func TestSmoke_PublisherOffer(t *testing.T) {
	cfg := requireRedis(t)
	env := testutil.NewIngestTestEnv(t, cfg)
	defer env.Close()

	token := registerAndGetToken(t, env)
	streamID := createStream(t, env, token, "webrtc-smoke", "publisher-peer-1")

	offerBody := `{"peer_id":"publisher-peer-1"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/streams/"+string(streamID)+"/publisher/offer", strings.NewReader(offerBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	env.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"sdp"`)
}

func TestSmoke_RedisStreamPersistence(t *testing.T) {
	cfg := requireRedis(t)

	env1 := testutil.NewIngestTestEnv(t, cfg)
	token := registerAndGetToken(t, env1)
	streamID := createStream(t, env1, token, "persist-stream", "owner-persist-1")
	env1.Close()

	env2 := testutil.NewIngestTestEnv(t, cfg)
	defer env2.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/streams/"+string(streamID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	env2.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "stream should persist in Redis after factory restart: %s", w.Body.String())
}

func TestSmoke_SignalWebSocketWithToken(t *testing.T) {
	cfg := requireRedis(t)

	ingest := testutil.NewIngestTestEnv(t, cfg)
	token := registerAndGetToken(t, ingest)
	ingest.Close()

	signal := testutil.NewSignalTestServer(t, cfg)
	defer signal.Close()

	wsURL := "ws" + strings.TrimPrefix(signal.Server.URL, "http") +
		"/ws?token=" + token + "&peer_id=ws-smoke-peer-1"

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v (http %v)", err, resp)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "i/o timeout") {
			t.Logf("optional read after connect: %v", err)
		}
	}
}

func TestSmoke_SignalReady(t *testing.T) {
	cfg := requireRedis(t)
	signal := testutil.NewSignalTestServer(t, cfg)
	defer signal.Close()

	resp, err := http.Get(signal.Server.URL + "/ready")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func registerAndGetToken(t *testing.T, env *testutil.IngestTestEnv) string {
	t.Helper()

	body := fmt.Sprintf(`{"username":"user%d","email":"user%d@smoke.test","password":"password123"}`, time.Now().UnixNano(), time.Now().UnixNano())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	env.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var reg map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reg))
	token, _ := reg["access_token"].(string)
	require.NotEmpty(t, token)
	return token
}

func createStream(t *testing.T, env *testutil.IngestTestEnv, token, name, ownerPeer string) domain.StreamID {
	t.Helper()

	payload := fmt.Sprintf(`{"name":%q,"owner":%q,"max_peers":50}`, name, ownerPeer)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/streams", bytes.NewReader([]byte(payload)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	env.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp struct {
		Stream struct {
			ID domain.StreamID `json:"id"`
		} `json:"stream"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Stream.ID)
	return resp.Stream.ID
}
