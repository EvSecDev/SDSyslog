package server

import (
	"sdsyslog/internal/global"
	"time"
)

const (
	ListenPortSender   int           = 10000 + global.DefaultReceiverPort // Default listen port
	ListenPortReceiver int           = 20000 + global.DefaultReceiverPort // Default listen port
	ListenAddr         string        = "localhost"                        // Metric queries only exposed to local machine
	ReadTimeout        time.Duration = 30 * time.Second
	WriteTimeout       time.Duration = 10 * time.Second
	IdleTimeout        time.Duration = 180 * time.Second

	DiscoverMode    string = "discover"
	DataMode        string = "data"
	AggregationMode string = "aggregation"
	BulkMode        string = "bulk"

	DiscoveryPath   string = "/" + DiscoverMode + "/"
	DataPath        string = "/" + DataMode + "/"
	AggregationPath string = "/" + AggregationMode + "/"
	BulkPath        string = "/" + BulkMode + "/"
)
