package bench

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/temporalio/es-bench/config"
	"github.com/urfave/cli/v2"
	"go.temporal.io/server/common/dynamicconfig"
	tlog "go.temporal.io/server/common/log"
	"go.temporal.io/server/common/metrics"
	"go.temporal.io/server/common/persistence/visibility"
	"go.temporal.io/server/common/persistence/visibility/manager"
	"go.temporal.io/server/common/persistence/visibility/store/elasticsearch/client"
	"go.temporal.io/server/common/searchattribute"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v3"
)

const (
	namespaceID = "default"
	workflowID  = "es-bench-workflow"
)

func NewBench(cfg *config.Config, done <-chan bool, index string) *Bench {
	return &Bench{
		cfg:       cfg,
		done:      done,
		indexName: index,
		logger:    tlog.NewZapLogger(tlog.BuildZapLogger(cfg.Log)),
	}
}

type Bench struct {
	indexName string
	logger    tlog.Logger
	cfg       *config.Config
	done      <-chan bool
}

func (b *Bench) IngestData(recordCount int, parallelFactor int) error {
	if b.indexName == "" {
		indexSuffix := randString(5)
		b.indexName = fmt.Sprintf("%s_%s", b.cfg.Elasticsearch.Indices[client.VisibilityAppName], indexSuffix)
	}

	//if err := b.createIndex(); err != nil {
	//	return cli.Exit(err, 1)
	//}
	fmt.Println("Created indexName", b.indexName)

	visibilityManager, err := b.newVisibilityManager()
	if err != nil {
		return cli.Exit(fmt.Sprintf("unable to create Elasticsearch visibility manager: %v", err), 1)
	}

	start := time.Now()
	wg := &sync.WaitGroup{}
	taskCount := recordCount / parallelFactor

	cfgBytes, _ := yaml.Marshal(b.cfg.Processor)
	fmt.Printf("=== Processor config:\n%s\n", cfgBytes)
	fmt.Printf("=== Input: %d records, with parallel factor: %d\n", recordCount, parallelFactor)

	errCount := &atomic.Int32{}
	requestCount := &atomic.Int32{}
	deletedCount := &atomic.Int32{}
	wg.Add(parallelFactor)
	for shardID := 0; shardID < parallelFactor; shardID++ {
		go func(shardID int32) {
			b.ingestionLoop(shardID, taskCount, visibilityManager,requestCount, deletedCount, errCount)
			wg.Done()
		}(int32(shardID))
	}

	wg.Wait()
	took := time.Since(start)

	fmt.Printf("=== Processor config:\n%s\n", cfgBytes)
	fmt.Printf("=== Input: %d records, with parallel factor: %d\n", recordCount, parallelFactor)
	fmt.Printf("=== Created/delete %d records, deleted %d, with %d errors, took %v, avg %v/record\n",
		requestCount.Load(),
		deletedCount.Load(),
		errCount.Load(),
		took,
		time.Duration(took.Nanoseconds()/int64(requestCount.Load())))

	return nil
}

func (b *Bench) ingestionLoop(
	shardID int32,
	taskCount int,
	visibilityManager manager.VisibilityManager,
	requestCount *atomic.Int32,
	deletedCount *atomic.Int32,
	errCount *atomic.Int32,
) {
	var lastStartReq *manager.RecordWorkflowExecutionStartedRequest
	for taskID := 0; taskID < taskCount; taskID++ {
		requestCount.Inc()
		if lastStartReq != nil && rand.Intn(10) < 1 {
			for {
				err1 := deleteWorkflow(visibilityManager, lastStartReq)
				if err1 != nil {
					errCount.Inc()
				} else {
					deletedCount.Inc()
					lastStartReq = nil
					break
				}
			}
		} else {
			request := generateRecordWorkflowExecutionStartedRequest(shardID, taskID)

			for {
				// Emulate task processor which retries forever.
				err1 := visibilityManager.RecordWorkflowExecutionStarted(request)
				if err1 != nil {
					fmt.Printf("failed to create ES record: %v", err1)
					time.Sleep(time.Second)
					errCount.Inc()
				} else {
					break
				}
			}

			lastStartReq = request
		}

		//// Emulate real load, not constant load. Add 2-4 seconds delay.
		//time.Sleep(time.Duration(rand.Intn(1000)+2000) * time.Millisecond)

		select {
		case <-b.done:
			fmt.Println("Done")
			return
		default:
		}
	}
}

func (b *Bench) QueryData(recordCount int, parallelFactor int, queryType string) error {
	visibilityManager, err := b.newVisibilityManager()
	if err != nil {
		return cli.Exit(fmt.Sprintf("unable to create Elasticsearch visibility manager: %v", err), 1)
	}

	start := time.Now()
	wg := &sync.WaitGroup{}

	pageSize := 100

	taskIDs := rand.Perm(recordCount / parallelFactor)

	fmt.Printf("=== Input: %d recordCount, with parallel factor: %d\n", recordCount, parallelFactor)

	requestCount := &atomic.Int32{}
	errCount := &atomic.Int32{}
	foundCount := &atomic.Int64{}
	wg.Add(parallelFactor)

	for shardID := 0; shardID < parallelFactor; shardID++ {
		go func(shardID int) {
			b.queryLoop(taskIDs, pageSize, queryType, visibilityManager, requestCount, errCount)
			wg.Done()
		}(shardID)
	}

	wg.Wait()
	took := time.Since(start)
	rps := float64(requestCount.Load()) / took.Seconds()

	fmt.Printf("=== Input: %d recordCount, with parallel factor: %d\n", recordCount, parallelFactor)
	fmt.Printf("=== Issued %d requests, with %d errors, %d results, took %v, RPS %f\n",
		requestCount,
		errCount.Load(),
		foundCount.Load(),
		took,
		rps)

	return nil
}

func (b *Bench) queryLoop(
	taskIDs []int,
	pageSize int,
	queryType string,
	visibilityManager manager.VisibilityManager,
	requestCount *atomic.Int32,
	errCount *atomic.Int32) {
	for taskID := range taskIDs {
		// request := &manager.ListWorkflowExecutionsRequestV2{
		// 	NamespaceID: namespaceID,
		// 	Query:       fmt.Sprintf(`CustomKeywordField="search-string-%d-%d"`, shardID, taskID),
		// 	PageSize: 1,
		// }
		_ = taskID
		request := &manager.ListWorkflowExecutionsRequestV2{
			NamespaceID: namespaceID,
			PageSize:    pageSize,
			Query: `WorkflowType = "test-workflow"`,
		}

		var err error
		if queryType == "list" {
			_, err = visibilityManager.ListWorkflowExecutions(request)
		} else if queryType == "scan" {
			_, err = visibilityManager.ScanWorkflowExecutions(request)
		} else {
			panic("unsupported query type " + queryType)
		}

		requestCount.Inc()

		if err != nil {
			errCount.Inc()
			fmt.Println(err)
		}

		select {
		case <-b.done:
			fmt.Println("Done")
			return
		default:
		}
	}
}

func (b *Bench) CreateIndex(indexName string) error {
	esClient, err := client.NewIntegrationTestsClient(b.cfg.Elasticsearch, b.logger)
	if err != nil {
		return fmt.Errorf("failed to create es client: %v", err)
	}

	ctx := context.Background()
	ok, err := esClient.CreateIndex(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to create es indexName: %v", err)
	}

	if !ok {
		return fmt.Errorf("CreateIndex request is not acknowledged")
	}

	return nil
}

func (b *Bench) DeleteIndex(indexName string) error {
	rawClient, err := newRawClient(b.cfg.Elasticsearch)
	if err != nil {
		return cli.Exit(fmt.Sprintf("unable to create raw ES client: %v", err), 1)
	}

	var indexesToDelete []string
	if indexName == "all" {
		resp, err := rawClient.CatIndices().Do(context.Background())
		if err != nil {
			return cli.Exit(fmt.Sprintf("unable to cat indexes: %v", err), 1)
		}
		for _, indexToDelete := range resp {
			indexesToDelete = append(indexesToDelete, indexToDelete.Index)
		}
	} else {
		indexesToDelete = append(indexesToDelete, indexName)
	}

	for _, indexToDelete := range indexesToDelete {
		fmt.Println("Deleting index", indexToDelete)
		resp, err := rawClient.DeleteIndex(indexToDelete).Do(context.Background())
		if err != nil || !resp.Acknowledged {
			return cli.Exit(fmt.Sprintf("unable to delete index: %v", err), 1)
		}
	}

	return nil
}

func (b *Bench) CreateSchemaTemplate(schemaTemplateFile string) error {
	template, err := ioutil.ReadFile(schemaTemplateFile)
	if err != nil {
		return err
	}
	templateName := fmt.Sprintf("%s_template", b.cfg.Elasticsearch.Indices[client.VisibilityAppName])
	fmt.Println("URL:", b.cfg.Elasticsearch.URL.String(), "template name:", templateName, "template:", strings.Replace(string(template)[:100], "\n", " ", -1))
	esClient, err := client.NewIntegrationTestsClient(b.cfg.Elasticsearch, b.logger)
	if err != nil {
		return err
	}
	ok, err := esClient.IndexPutTemplate(context.Background(), templateName, string(template))
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("IndexPutTemplate request is not acknowledged")
	}

	return nil
}

func (b *Bench) newVisibilityManager() (manager.VisibilityManager, error) {
	esClient, err := client.NewClient(b.cfg.Elasticsearch, nil, b.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create es client: %v", err)
	}

	metricsScope := b.cfg.Metrics.NewScope(b.logger)
	metricsClient := metrics.NewClient(&metrics.ClientConfig{}, metricsScope, metrics.History)

	return visibility.NewAdvancedManager(
		b.indexName,
		esClient,
		config.ProcessorConfig(b.cfg),
		searchattribute.NewTestProvider(),
		nil,
		dynamicconfig.GetIntPropertyFn(100000),
		dynamicconfig.GetIntPropertyFn(100000),
		metricsClient,
		b.logger)
}

func deleteWorkflow(m manager.VisibilityManager, startReq *manager.RecordWorkflowExecutionStartedRequest) error {
	deleteRequest := &manager.VisibilityDeleteWorkflowExecutionRequest{
		NamespaceID: startReq.NamespaceID,
		WorkflowID:  startReq.Execution.GetWorkflowId(),
		RunID:       startReq.Execution.GetRunId(),
		TaskID:      math.MaxInt,
	}
	err := m.DeleteWorkflowExecution(deleteRequest)
	return err
}
