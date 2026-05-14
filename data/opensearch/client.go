package opensearch

import (
	"errors"
	"os"

	opensearchCrud "github.com/alec404/go-crud/opensearch"
	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
)

// NewClient creates an OpenSearch client from bootstrap config.
func NewClient(logger log.Logger, cfg *conf.Bootstrap, opts ...opensearchCrud.Option) (*opensearchCrud.Client, error) {
	if cfg == nil || cfg.Data == nil || cfg.Data.Opensearch == nil {
		return nil, errors.New("opensearch config is nil")
	}

	openSearchCfg := cfg.Data.Opensearch
	options := make([]opensearchCrud.Option, 0, len(opts)+12)

	if logger != nil {
		options = append(options, opensearchCrud.WithLogger(logger))
	}
	if len(openSearchCfg.GetAddresses()) > 0 {
		options = append(options, opensearchCrud.WithAddresses(openSearchCfg.GetAddresses()...))
	}
	if openSearchCfg.GetUsername() != "" {
		options = append(options, opensearchCrud.WithUsername(openSearchCfg.GetUsername()))
	}
	if openSearchCfg.GetPassword() != "" {
		options = append(options, opensearchCrud.WithPassword(openSearchCfg.GetPassword()))
	}

	if len(openSearchCfg.GetRetryOnStatus()) > 0 {
		retryOnStatus := make([]int, 0, len(openSearchCfg.GetRetryOnStatus()))
		for _, status := range openSearchCfg.GetRetryOnStatus() {
			retryOnStatus = append(retryOnStatus, int(status))
		}
		options = append(options, opensearchCrud.WithRetryOnStatus(retryOnStatus...))
	}
	if openSearchCfg.DisableRetry != nil {
		options = append(options, opensearchCrud.WithDisableRetry(openSearchCfg.GetDisableRetry()))
	}
	if openSearchCfg.EnableRetryOnTimeout != nil {
		options = append(options, opensearchCrud.WithEnableRetryOnTimeout(openSearchCfg.GetEnableRetryOnTimeout()))
	}
	if openSearchCfg.MaxRetries != nil {
		options = append(options, opensearchCrud.WithMaxRetries(int(openSearchCfg.GetMaxRetries())))
	}

	if openSearchCfg.CompressRequestBody != nil {
		options = append(options, opensearchCrud.WithCompressRequestBody(openSearchCfg.GetCompressRequestBody()))
	}

	if openSearchCfg.DiscoverNodesOnStart != nil {
		options = append(options, opensearchCrud.WithDiscoverNodesOnStart(openSearchCfg.GetDiscoverNodesOnStart()))
	}
	if openSearchCfg.DiscoverNodesInterval != nil {
		options = append(options, opensearchCrud.WithDiscoverNodesInterval(openSearchCfg.GetDiscoverNodesInterval().AsDuration()))
	}

	if openSearchCfg.EnableMetrics != nil {
		options = append(options, opensearchCrud.WithEnableMetrics(openSearchCfg.GetEnableMetrics()))
	}
	if openSearchCfg.EnableDebugLogger != nil {
		options = append(options, opensearchCrud.WithEnableDebugLogger(openSearchCfg.GetEnableDebugLogger()))
	}

	if openSearchCfg.Tls != nil {
		caData, err := loadCACertData(openSearchCfg.Tls)
		if err != nil {
			return nil, err
		}
		if len(caData) > 0 {
			options = append(options, opensearchCrud.WithCACert(caData))
		}
	}

	if opts != nil {
		options = append(options, opts...)
	}

	return opensearchCrud.NewOpenSearchClient(options...)
}

// NewOpenSearchClient keeps the naming style used by the existing data helpers.
func NewOpenSearchClient(cfg *conf.Bootstrap, logger log.Logger, opts ...opensearchCrud.Option) (*opensearchCrud.Client, error) {
	return NewClient(logger, cfg, opts...)
}

func loadCACertData(cfg *conf.TLS) ([]byte, error) {
	if cfg == nil {
		return nil, nil
	}

	if cfg.File != nil {
		caPath := cfg.File.GetCaPath()
		if caPath == "" {
			return nil, errors.New("CA path is empty")
		}
		return os.ReadFile(caPath)
	}

	if cfg.Config != nil {
		caData := cfg.Config.GetCaPem()
		if len(caData) == 0 {
			return nil, errors.New("CA PEM is empty")
		}
		return caData, nil
	}

	return nil, errors.New("invalid TLS config: no CA certificate found")
}
