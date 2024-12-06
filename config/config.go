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
Package config 설정 패키지
*/
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// 빌드 시 설정됨
var (
	Version   = "unknown"
	BuildTime = "unknown"
)

const (
	ModuleName   = "weblin"
	PidFilePath  = "var/.weblin.pid"
	LogFilePath  = "log/weblin.log"
	ConfFilePath = "conf/weblin.yaml"
)

// Config 설정 정보 구조체
type Config struct {
	// 서버 설정
	Server struct {
		// 서버 리스닝 포트 (DEF:8443)
		Port int `yaml:"port"`
		// TLS 설정
		TLS TLSYaml `yaml:"tls"`
	} `yaml:"server"`

	// API 설정
	API struct {
		// 서버 메트릭을 제공하는 엔드포인트 (DEF:/metrics)
		MetricURI string `yaml:"metricURI"`
		// 서버 상태 점검을 위한 엔드포인트 (DEF:/health)
		HealthURI string `yaml:"healthURI"`
		// 서버 상태 정보를 제공하는 엔드포인트 (DEF:/sys/stats)
		SysStatURI string `yaml:"sysStatURI"`
	} `yaml:"api"`

	// 로그 설정
	Log struct {
		// 최대 로그 파일 사이즈 (DEF:100MB, MIN:1MB, MAX:1000MB)
		MaxLogFileSize int `yaml:"maxLogFileSize"`
		// 최대 로그 파일 백업 개수 (DEF:10, MIN:1, MAX:100)
		MaxLogFileBackup int `yaml:"maxLogFileBackup"`
		// 최대 백업 로그 파일 유지 기간(일) (DEF:90, MIN:1, MAX:365)
		MaxLogFileAge int `yaml:"maxLogFileAge"`
		// 백업 로그 파일 압축 여부 (DEF:true, ENABLE:true, DISABLE:false)
		CompBakLogFile bool `yaml:"compressBackupLogFile"`
	} `yaml:"log"`
}

// TLSYaml TLS 설정 YAML 구조체
type TLSYaml struct {
	// TLS 사용 설정 (DEF:false)
	Enabled bool `yaml:"enabled"`
	// TLS Certificate Path
	TLSCertPath string `yaml:"tlsCertPath"`
	// TLS Private Key Path
	TLSKeyPath string `yaml:"tlsKeyPath"`
}

// RunConfig 런타임 설정 정보 구조체
type RunConfig struct {
	DebugMode bool
	Pid       int
}

var RunConf RunConfig
var Conf Config

// 패키지 임포트 시 초기화
func init() {
	Conf.Server.Port = 8443
	Conf.API.MetricURI = "/metrics"
	Conf.API.HealthURI = "/health"
	Conf.API.SysStatURI = "/sys/stats"
	Conf.Log.MaxLogFileSize = 100
	Conf.Log.MaxLogFileBackup = 10
	Conf.Log.MaxLogFileAge = 90
	Conf.Log.CompBakLogFile = true
}

// LoadConfig 설정 파일 로드
//
// Parameters:
//   - filePath: 설정 파일 경로
//
// Returns:
//   - error: 성공(nil), 실패(error)
func (c *Config) LoadConfig(filePath string) error {
	// YAML 설정 파일 열기
	file, err := os.Open(ConfFilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// YAML 디코더 생성
	decoder := yaml.NewDecoder(file)

	// YAML 파싱 및 디코딩
	err = decoder.Decode(&Conf)
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	// 설정 값 유효성 검사
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		c.Server.Port = 8443
	}
	if c.Log.MaxLogFileSize < 1 || c.Log.MaxLogFileSize > 1000 {
		c.Log.MaxLogFileSize = 100
	}
	if c.Log.MaxLogFileBackup < 1 || c.Log.MaxLogFileBackup > 100 {
		c.Log.MaxLogFileBackup = 10
	}
	if c.Log.MaxLogFileAge < 1 || c.Log.MaxLogFileAge > 365 {
		c.Log.MaxLogFileAge = 90
	}

	return nil
}
