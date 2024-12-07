// Copyright 2024 Weblin Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

/*
Package server weblin 메인 서버 패키지
*/
package server

import (
	"context"
	"crypto/tls"
	"net/http"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meloncoffee/weblin/config"
	"github.com/meloncoffee/weblin/internal/logger"
	"github.com/meloncoffee/weblin/pkg/utils/file"
	"github.com/meloncoffee/weblin/pkg/utils/process"
	"github.com/thoas/stats"
)

var (
	doOnce sync.Once
	// 서버 응답 시간 및 상태 코드 카운트
	servStats *stats.Stats
)

type Server struct{}

// Run 메인 서버 가동
//
// Parameters:
//   - ctx: 서버 종료 컨텍스트
func (s *Server) Run(ctx context.Context) {
	var tlsConf tls.Config
	var err error
	isTLS := false
	port := config.Conf.Server.Port

	if config.Conf.Server.TLS.Enabled {
		// TLS 인증서 및 키 파일 유효성 검사
		tlsCertPath := config.Conf.Server.TLS.TLSCertPath
		if tlsCertPath == "" || !file.IsFileExists(tlsCertPath) {
			logger.Log.LogError("Not found TLS certificate (cert path: %s)", tlsCertPath)
			process.SendSignal(config.RunConf.Pid, syscall.SIGUSR1)
			return
		}
		tlsKeyPath := config.Conf.Server.TLS.TLSKeyPath
		if tlsKeyPath == "" || !file.IsFileExists(tlsKeyPath) {
			logger.Log.LogError("Not found TLS key (key path: %s)", tlsKeyPath)
			process.SendSignal(config.RunConf.Pid, syscall.SIGUSR1)
			return
		}

		// TLS 설정
		if tlsConf.NextProtos == nil {
			// 애플리케이션 계층 프로토콜(HTTP/1.1, HTTP/2) 설정
			tlsConf.NextProtos = []string{"h2", "http/1.1"}
		}

		// TLS 인증서 파일 로드
		tlsConf.Certificates = make([]tls.Certificate, 1)
		tlsConf.Certificates[0], err = tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
		if err != nil {
			logger.Log.LogError("Failed to load TLS certificate: %v", err)
			process.SendSignal(config.RunConf.Pid, syscall.SIGUSR1)
			return
		}

		isTLS = true
	}

	// HTTP 서버 설정
	server := &http.Server{
		Addr: ":" + strconv.Itoa(port),
		// gin 엔진 설정
		Handler: s.newGinRouterEngine(),
		// 요청 타임아웃 10초 설정
		ReadTimeout: 10 * time.Second,
		// 응답 타임아웃 10초 설정
		WriteTimeout: 10 * time.Second,
		// 요청 헤더 최대 크기를 1MB로 설정
		MaxHeaderBytes: 1 << 20,
	}

	// HTTP 서버 가동
	if isTLS {
		server.TLSConfig = &tlsConf
		go func() {
			err := server.ListenAndServeTLS("", "")
			if err != nil && err != http.ErrServerClosed {
				logger.Log.LogError("Server error occurred: %v", err)
				process.SendSignal(config.RunConf.Pid, syscall.SIGUSR1)
			}
		}()
	} else {
		go func() {
			err := server.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				logger.Log.LogError("Server error occurred: %v", err)
				process.SendSignal(config.RunConf.Pid, syscall.SIGUSR1)
			}
		}()
	}

	logger.Log.LogInfo("Server listening on port %d", port)

	// 서버 종료 신호 대기
	<-ctx.Done()

	// 종료 신호를 받았으면 graceful shutdown을 위해 5초 타임아웃 설정
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 서버 종료
	err = server.Shutdown(shutdownCtx)
	if err != nil {
		logger.Log.LogWarn("Server shutdown: %v", err)
		return
	}

	logger.Log.LogInfo("Server shutdown on port %d", port)
}

// newRouterEngine gin 엔진 생성
//
// Returns:
//   - *gin.Engine: gin 엔진
func (s *Server) newGinRouterEngine() *gin.Engine {
	// 런타임 중 한번만 호출됨
	doOnce.Do(func() {
		// Stats 구조체 생성
		servStats = stats.New()
	})

	// gin 동작 모드 설정
	gin.SetMode(func() string {
		if config.RunConf.DebugMode {
			return gin.DebugMode
		}
		return gin.ReleaseMode
	}())

	// gin 라우터 생성
	r := gin.New()

	// 복구 미들웨어 등록
	r.Use(gin.Recovery())
	// 요청/응답 정보 로깅 미들웨어 등록
	r.Use(s.ginLoggerMiddleware())
	// 버전 정보 미들웨어 등록
	r.Use(s.versionMiddleware())
	// 요청 통계를 수집하고 기록하는 미들웨어 등록
	r.Use(s.statMiddleware())

	// 요청 핸들러 등록
	r.GET(config.Conf.API.MetricURI, metricsHandler)
	r.GET(config.Conf.API.HealthURI, healthHandler)
	r.GET(config.Conf.API.SysStatURI, sysStatsHandler)
	r.GET("/version", versionHandler)
	r.GET("/", rootHandler)

	return r
}

// ginLoggerMiddleware gin 요청/응답 정보 로깅 미들웨어
//
// Returns:
//   - gin.HandlerFunc: gin 미들웨어
func (s *Server) ginLoggerMiddleware() gin.HandlerFunc {
	// 로깅에서 제외할 경로 설정
	excludePath := map[string]struct{}{
		config.Conf.API.MetricURI: {},
		config.Conf.API.HealthURI: {},
	}

	return func(c *gin.Context) {
		// 요청 시작 시간 획득
		start := time.Now()
		// 요청 경로 획득
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		// 요청 처리
		c.Next()

		// 제외할 경로는 로깅하지 않음
		if _, ok := excludePath[path]; ok {
			return
		}

		// 요청 종료 시간 및 latency 계산
		end := time.Now()
		latency := end.Sub(start)

		// 로그 메시지 설정
		var logMsg string
		if len(c.Errors) > 0 {
			logMsg = c.Errors.String()
		} else {
			logMsg = "Request"
		}
		// 상태 코드 획득
		statusCode := c.Writer.Status()
		// 요청 메서드 획득
		method := c.Request.Method
		// 요청 클라이언트 IP 획득
		clientIP := c.ClientIP()
		// 사용자 에이전트 획득
		userAgent := c.Request.UserAgent()
		// 응답 바디 사이즈 획득
		resBodySize := c.Writer.Size()

		// 로그 출력 (상태 코드에 따른 로그 레벨 설정)
		if statusCode >= 500 {
			logger.Log.LogError("[%d] %s %s (IP: %s, Latency: %v, UA: %s, ResSize: %d) %s",
				statusCode, method, path, clientIP, latency, userAgent, resBodySize, logMsg)
		} else if statusCode >= 400 {
			logger.Log.LogWarn("[%d] %s %s (IP: %s, Latency: %v, UA: %s, ResSize: %d) %s",
				statusCode, method, path, clientIP, latency, userAgent, resBodySize, logMsg)
		} else {
			logger.Log.LogInfo("[%d] %s %s (IP: %s, Latency: %v, UA: %s, ResSize: %d) %s",
				statusCode, method, path, clientIP, latency, userAgent, resBodySize, logMsg)
		}
	}
}

// versionMiddleware 버전 정보 미들웨어
//
// Returns:
//   - gin.HandlerFunc: gin 미들웨어
func (s *Server) versionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-WEBLIN-VERSION", config.Version)
		c.Next()
	}
}

// statMiddleware 요청 통계를 수집하고 기록하는 미들웨어
//
// Returns:
//   - gin.HandlerFunc: gin 미들웨어
func (s *Server) statMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		beginning, recorder := servStats.Begin(c.Writer)
		c.Next()
		servStats.End(beginning, stats.WithRecorder(recorder))
	}
}
