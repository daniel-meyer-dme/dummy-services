package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// FailService is a simple interface for a service that implements 3 endpoints
// The /health EP responds with the health status of the service
// The /sethealthy EP switches the health state of the service to healthy
// The /setunhealthy EP switches the health state of the service to unhealthy
type FailService interface {
	HealthEndpointHandler(w http.ResponseWriter, r *http.Request)
	SetHealthyEndpointHandler(w http.ResponseWriter, r *http.Request)
	SetUnHealthyEndpointHandler(w http.ResponseWriter, r *http.Request)
	OomKillEndpointHandler(w http.ResponseWriter, r *http.Request)
	Start()
	Stop()
}

var errorResponseWrongMethod = []byte("{ \"error\": \"Invalid method used. You have to use the PUT method.\" }")

type healthResponse struct {
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
}

type failServiceImpl struct {
	healthyFor            int64
	healthyIn             int64
	unHealthyFor          int64
	ticker                *time.Ticker
	healthy               bool
	changeStateAt         int64
	wasHealthyOnce        bool
	overwrittenByEndpoint bool
	oomAfter              int64
}

func validateHTTPMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPut {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errorResponseWrongMethod)
		return false
	}

	return true
}

func (fs *failServiceImpl) SetHealthyEndpointHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("SetHealthyEndpointHandler called")

	if validateHTTPMethod(w, r) {
		fs.overwrittenByEndpoint = true
		fs.healthy = true
	}
}

func (fs *failServiceImpl) SetUnHealthyEndpointHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("SetUnHealthyEndpointHandler called")

	if validateHTTPMethod(w, r) {
		fs.overwrittenByEndpoint = true
		fs.healthy = false
	}
}

func (fs *failServiceImpl) OomKillEndpointHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("OomKillEndpointHeader called")

	if validateHTTPMethod(w, r) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Force OOM Now"))
		forceOomKill()
	}
}

func forceOomKill() {
	massiveMemory := make(map[string]string)

	for {
		massiveMemory[RandStringBytesMaskImprSrc(128)] = RandStringBytesMaskImprSrc(128)

	}
	log.Print(massiveMemory)
}

var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

//RandStringBytesMaskImprSrc - generate random string using masking with source
func RandStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	l := len(letterBytes)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < l {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func (fs *failServiceImpl) HealthEndpointHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HealthEndpointHandler called")

	healthResponse := healthResponse{
		Message: "Ok",
		Ok:      true,
	}

	w.Header().Set("Content-Type", "application/json")
	if fs.healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusGatewayTimeout)
		healthResponse.Ok = false
		healthResponse.Message = "Error"
	}

	err := json.NewEncoder(w).Encode(healthResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (fs *failServiceImpl) Stop() {
	fs.ticker.Stop()
}

func stateToStr(healthy bool) string {
	state := "healthy"
	if !healthy {
		state = "unhealthy"
	}
	return state
}

func (fs *failServiceImpl) Start() {

	currentTime := time.Now().Unix()
	fs.changeStateAt = fs.nextEvalStateChange(currentTime)

	fs.ticker = time.NewTicker(time.Millisecond * 1000)

	if fs.oomAfter > 0 {
		go func() {
			duration := time.Duration(time.Second * time.Duration(fs.oomAfter))
			ticker := time.NewTicker(duration)
			log.Printf("Going oom in %s", duration)
			for {

				select {
				case <-ticker.C:
					log.Print("Going oom")
					ticker.Stop()
					forceOomKill()
				}
			}
		}()
	}
	go func() {
		for range fs.ticker.C {

			if !fs.overwrittenByEndpoint {
				currentTime := time.Now().Unix()
				if fs.isChangeState(currentTime) {
					fs.switchHealthy()
					fs.changeStateAt = fs.nextEvalStateChange(currentTime)

					log.Printf("State changed")
					log.Printf("Next state change at %s", time.Unix(fs.changeStateAt, 0).String())
				}
			}

			overwrittenByEndpointStr := ""
			if fs.overwrittenByEndpoint {
				overwrittenByEndpointStr = " - Was set and thus fixed by endpoint."
			}
			log.Printf("State %s %s", stateToStr(fs.healthy), overwrittenByEndpointStr)
		}
	}()
}

func (fs *failServiceImpl) switchHealthy() {
	if !fs.wasHealthyOnce && fs.healthy {
		fs.wasHealthyOnce = true
	}
	fs.healthy = !fs.healthy
}

func (fs *failServiceImpl) isChangeState(currentTime int64) bool {

	if currentTime > fs.changeStateAt {
		return true
	}
	return false
}

func (fs *failServiceImpl) nextEvalStateChange(currentTime int64) int64 {

	// currently healthy ... stay healthy for ...
	if fs.healthy {
		return currentTime + fs.healthyFor
	}

	// in case healthyIn is -1 or negative at all
	healthyIn := fs.healthyIn
	if healthyIn < 0 {
		healthyIn = 0
	}

	// currently not healthy + were never healthy before ... initially get healthy
	if !fs.wasHealthyOnce {
		return currentTime + healthyIn
	}

	// currently not healthy ... stay unhealthy for ...
	return currentTime + fs.unHealthyFor
}

// NewFailService creates a new instance of a FailService implementation
func NewFailService(healthyIn int64, healthyFor int64, unHealthyFor int64, oomAfter int64) FailService {

	healthy := false
	// immediately start healthy
	if healthyIn == 0 {
		healthy = true
	}

	result := &failServiceImpl{
		healthyIn:             healthyIn,
		healthyFor:            healthyFor,
		unHealthyFor:          unHealthyFor,
		oomAfter:              oomAfter,
		healthy:               healthy,
		wasHealthyOnce:        healthy,
		changeStateAt:         0,
		overwrittenByEndpoint: false,
	}

	return result
}
