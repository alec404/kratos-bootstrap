package elasticsearch

import (
	"fmt"
	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/olivere/elastic"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"
)

type ElasticLoggerAdapter struct {
	logger log.Logger
}

func (l *ElasticLoggerAdapter) Printf(format string, v ...interface{}) {
	_ = l.logger.Log(log.LevelDebug, "msg", fmt.Sprintf(format, v...))
}

func NewElasticsearchClient(c *conf.Bootstrap, logger log.Logger) *elastic.Client {
	transport := http.DefaultTransport.(*http.Transport)

	clientOptions := []elastic.ClientOptionFunc{
		elastic.SetHttpClient(&http.Client{
			Transport: otelhttp.NewTransport(transport),
		}),
		elastic.SetURL(c.Data.Elasticsearch.Addresses...),
		elastic.SetBasicAuth(c.Data.Elasticsearch.Username, c.Data.Elasticsearch.Password),
		elastic.SetSniff(c.Data.Elasticsearch.EnableSniffer),
		elastic.SetHealthcheck(c.Data.Elasticsearch.EnableHealth),
		elastic.SetGzip(c.Data.Elasticsearch.EnableGzip),
	}
	if c.Data.Elasticsearch.Debug {
		clientOptions = append(clientOptions, elastic.SetTraceLog(&ElasticLoggerAdapter{logger: logger}))
	}

	client, err := elastic.NewClient(clientOptions...)

	if err != nil {
		log.Fatalf("failed opening connection to es: %v", err)
	}
	return client
}
