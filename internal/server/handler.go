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

package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/meloncoffee/weblin/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsHandler prometheus 메트릭 제공 핸들러
//
// Parameters:
//   - c: HTTP 요청 및 응답과 관련된 정보를 포함하는 객체
func metricsHandler(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

// healthHandler 헬스 체크 핸들러
//
// Parameters:
//   - c: HTTP 요청 및 응답과 관련된 정보를 포함하는 객체
func healthHandler(c *gin.Context) {
	c.AbortWithStatus(http.StatusOK)
}

// sysStatsHandler 서버 상태 정보 핸들러
//
// Parameters:
//   - c: HTTP 요청 및 응답과 관련된 정보를 포함하는 객체
func sysStatsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, servStats.Data())
}

// versionHandler 버전 정보 핸들러
//
// Parameters:
//   - c: HTTP 요청 및 응답과 관련된 정보를 포함하는 객체
func versionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"source":  "https://github.com/meloncoffee/weblin",
		"version": config.Version,
	})
}

// rootHandler 루트 경로 핸들러
//
// Parameters:
//   - c: HTTP 요청 및 응답과 관련된 정보를 포함하는 객체
func rootHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"text": "Welcome to weblin.",
	})
}
