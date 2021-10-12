package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/olivere/elastic/v7"
	"github.com/pborman/uuid"
	"github.com/urfave/cli/v2"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/server/common/dynamicconfig"
	tlog "go.temporal.io/server/common/log"
	"go.temporal.io/server/common/metrics"
	"go.temporal.io/server/common/payload"
	"go.temporal.io/server/common/persistence/visibility"
	"go.temporal.io/server/common/persistence/visibility/manager"
	"go.temporal.io/server/common/persistence/visibility/store/elasticsearch/client"
	"go.temporal.io/server/common/searchattribute"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v3"

	"github.com/temporalio/es-bench/config"
)

const (
	namespaceID = "default"
	workflowID  = "es-bench-workflow"
)

func main() {
	app := buildCLI()
	_ = app.Run(os.Args)
}

// buildCLI is the main entry point for the temporal server
func buildCLI() *cli.App {
	app := cli.NewApp()
	app.Name = "es-bench"
	app.Usage = "Temporal ES benchmark analyzer"
	app.ArgsUsage = " "
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "env",
			Value: "development",
			Usage: "running environment: development or production",
		},
		&cli.StringFlag{
			Name:  "es-password",
			Value: "",
			Usage: "Elasticsearch password (override config)",
		},
	}
	ctrlC := make(chan os.Signal, 1)
	signal.Notify(ctrlC, os.Interrupt, syscall.SIGTERM)
	exit := false
	go func() {
		select {
		case <-ctrlC:
			exit = true
		}
	}()

	rand.Seed(time.Now().UnixNano())

	app.Commands = []*cli.Command{
		// {
		// 	Name: "raw",
		// 	Flags: []cli.Flag{
		// 		&cli.StringFlag{
		// 			Name:     "mode",
		// 			Required: true,
		// 			Usage:    "attr or label",
		// 		},
		// 	},
		// 	Before: func(c *cli.Context) error {
		// 		return nil
		// 	},
		// 	Action: func(c *cli.Context) error {
		// 		cfg, err := config.LoadConfig("development", "./config", "")
		// 		if err != nil {
		// 			return cli.Exit(fmt.Sprintf("unable to load configuration: %v", err), 1)
		// 		}
		//
		// 		err = createSchemaTemplate(cfg, "./schema/index_template.json")
		// 		if err != nil {
		// 			return cli.Exit(fmt.Sprintf("unable to create index template: %v", err), 1)
		// 		}
		//
		// 		indexName, err := createIndex(cfg, randString(5))
		// 		if err != nil {
		// 			return cli.Exit(fmt.Sprintf("unable to create index: %v", err), 1)
		// 		}
		// 		fmt.Println("Created index", indexName)
		//
		// 		rawClient, err := newRawClient(cfg.Elasticsearch)
		// 		if err != nil {
		// 			return cli.Exit(fmt.Sprintf("unable to create raw ES client: %v", err), 1)
		// 		}
		//
		// 		start := time.Now()
		// 		wg := &sync.WaitGroup{}
		// 		recordCount := 100000
		// 		parallelFactor := 1000
		// 		errCount := atomic.Int32{}
		// 		wg.Add(parallelFactor)
		//
		// 		for shardID := 0; shardID < parallelFactor; shardID++ {
		// 			go func(shardID int32) {
		// 				for taskID := 0; taskID < recordCount/parallelFactor; taskID++ {
		// 					request := generateRecordWorkflowExecutionStartedRequest(shardID, taskID)
		// 					_, err1 := rawClient.Index().
		// 						Index(indexName).
		// 						Id(getDocID(request.Execution.WorkflowId, request.Execution.RunId)).
		// 						Version(request.TaskID).
		// 						VersionType("external").
		// 						BodyJson(
		// 							generateESDoc(
		// 								request,
		// 								getVisibilityTaskKey(shardID, taskID),
		// 								searchattribute.TestNameTypeMap,
		// 							)).Do(context.Background())
		// 					if err1 != nil {
		// 						errCount.Inc()
		// 					}
		// 				}
		// 				wg.Done()
		// 			}(int32(shardID))
		// 		}
		//
		// 		wg.Wait()
		// 		took := time.Since(start)
		// 		fmt.Printf("Created %d records, with %d errors, took %v, avg %v/record\n",
		// 			recordCount,
		// 			errCount.Load(),
		// 			took,
		// 			time.Duration(took.Nanoseconds()/int64(recordCount)))
		//
		// 		return cli.Exit("done!", 0)
		// 	},
		// },
		//
		// {
		// 	Name: "raw-bulk",
		// 	Flags: []cli.Flag{
		// 		&cli.StringFlag{
		// 			Name:     "mode",
		// 			Required: true,
		// 			Usage:    "attr or label",
		// 		},
		// 		&cli.IntFlag{
		// 			Name:  "num-indexes",
		// 			Value: 1,
		// 			Usage: "number of indexes to create and use",
		// 		},
		// 	},
		// 	Before: func(c *cli.Context) error {
		// 		return nil
		// 	},
		// 	Action: func(c *cli.Context) error {
		// 		cfg, err := config.LoadConfig("development", "./config", "")
		// 		if err != nil {
		// 			return cli.Exit(fmt.Sprintf("unable to load configuration: %v", err), 1)
		// 		}
		//
		// 		err = createSchemaTemplate(cfg, "./schema/index_template.json")
		// 		if err != nil {
		// 			return cli.Exit(fmt.Sprintf("unable to create index template: %v", err), 1)
		// 		}
		//
		// 		rawClient, err := newRawClient(cfg.Elasticsearch)
		// 		if err != nil {
		// 			return cli.Exit(fmt.Sprintf("unable to create raw ES client: %v", err), 1)
		// 		}
		//
		// 		indexNames, err := createIndexRaw(rawClient, c.Int("num-indexes"))
		// 		if err != nil {
		// 			return cli.Exit(fmt.Sprintf("unable to create index: %v", err), 1)
		// 		}
		// 		fmt.Printf("Created %d indexes\n", len(indexNames))
		// 		if len(indexNames) < 11 {
		// 			fmt.Println(indexNames)
		// 		}
		//
		// 		start := time.Now()
		// 		wg := &sync.WaitGroup{}
		// 		recordCount := 500000
		// 		parallelFactor := 1000
		// 		errCount := atomic.Int32{}
		// 		requestCount := atomic.Int32{}
		// 		wg.Add(parallelFactor)
		//
		// 		esBulkProcessor, err := rawClient.BulkProcessor().
		// 			Name("processor").
		// 			Workers(1).
		// 			BulkSize(2 << 24). // 16MB
		// 			FlushInterval(time.Second).
		// 			After(func(executionId int64, requests []elastic.BulkableRequest, response *elastic.BulkResponse, err error) {
		// 				if err != nil {
		// 					fmt.Println(err)
		// 					errCount.Inc()
		// 				}
		// 			}).
		// 			Do(context.Background())
		//
		// 		for shardID := 0; shardID < parallelFactor; shardID++ {
		// 			go func(shardID int32) {
		// 				for taskID := 0; taskID < recordCount/parallelFactor; taskID++ {
		// 					request := generateRecordWorkflowExecutionStartedRequest(shardID, taskID)
		// 					indexName := indexNames[rand.Intn(len(indexNames))]
		// 					bulkRequest := elastic.NewBulkIndexRequest().
		// 						Index(indexName).
		// 						// Id(getDocID(request.Execution.WorkflowId, request.Execution.RunId)).
		// 						Id(getVisibilityTaskKey(shardID, taskID)).
		// 						VersionType("external").
		// 						Version(request.TaskID).
		// 						Doc(generateESDoc(
		// 							request,
		// 							getVisibilityTaskKey(shardID, taskID),
		// 							searchattribute.TestNameTypeMap,
		// 						))
		// 					esBulkProcessor.Add(bulkRequest)
		// 					requestCount.Inc()
		// 					if exit {
		// 						break
		// 					}
		// 				}
		// 				wg.Done()
		// 			}(int32(shardID))
		// 		}
		//
		// 		wg.Wait()
		// 		took := time.Since(start)
		// 		fmt.Printf("Created %d records, with %d errors, took %v, avg %v/record\n",
		// 			requestCount.Load(),
		// 			errCount.Load(),
		// 			took,
		// 			time.Duration(took.Nanoseconds()/int64(requestCount.Load())))
		//
		// 		return cli.Exit("done!", 0)
		// 	},
		// },

		{
			Name: "manager",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "index",
					Value: "",
					Usage: "index name",
				},
				&cli.IntFlag{
					Name:  "records",
					Value: 1_000_000,
					Usage: "records count",
				},
				&cli.IntFlag{
					Name:  "pf",
					Value: 1_000,
					Usage: "parallel factor",
				},
				&cli.IntFlag{
					Name:  "bulk-actions",
					Usage: "BulkActions config override",
				},
				&cli.DurationFlag{
					Name:  "flush-interval",
					Usage: "FlushInterval config override",
				},
			},
			Action: func(c *cli.Context) error {
				cfg, err := config.LoadConfig(c.String("env"), "./config", "")
				if err != nil {
					return cli.Exit(fmt.Sprintf("unable to load configuration: %v", err), 1)
				}

				if c.IsSet("es-password") && c.String("es-password") != "" {
					cfg.Elasticsearch.Password = c.String("es-password")
				}
				if c.IsSet("bulk-actions") {
					cfg.Processor.BulkActions = c.Int("bulk-actions")
				}
				if c.IsSet("flush-interval") {
					cfg.Processor.FlushInterval = c.Duration("flush-interval")
				}

				// err = createSchemaTemplate(cfg, "./schema/index_template.json")
				// if err != nil {
				// 	return cli.Exit(fmt.Sprintf("unable to create index template: %v", err), 1)
				// }

				indexName := c.String("index")
				if indexName == "" {
					indexName, err = createIndex(cfg, randString(5))
					if err != nil {
						return cli.Exit(fmt.Sprintf("unable to create index: %v", err), 1)
					}
					fmt.Println("Created index", indexName)
				}

				visibilityManager, err := newVisibilityManager(cfg, indexName)
				if err != nil {
					return cli.Exit(fmt.Sprintf("unable to create Elasticsearch visibility manager: %v", err), 1)
				}

				start := time.Now()
				wg := &sync.WaitGroup{}
				recordCount := c.Int("records")
				parallelFactor := c.Int("pf")
				taskCount := recordCount / parallelFactor

				cfgBytes, _ := yaml.Marshal(cfg.Processor)
				fmt.Printf("=== Processor config:\n%s\n", cfgBytes)
				fmt.Printf("=== Input: %d records, with parallel factor: %d\n", recordCount, parallelFactor)

				errCount := atomic.Int32{}
				requestCount := atomic.Int32{}
				deletedCount := atomic.Int32{}
				runIDsToDelete := make([]string, parallelFactor)
				wg.Add(parallelFactor)
				for shardID := 0; shardID < parallelFactor; shardID++ {
					go func(shardID int32) {
						for taskID := 0; taskID < taskCount; taskID++ {
							requestCount.Inc()
							if rand.Intn(10) < 1 {
								if runIDsToDelete[shardID] != "" {
									for {
										err1 := deleteWorkflow(visibilityManager, runIDsToDelete[shardID])
										if err1 != nil {
											errCount.Inc()
										} else {
											deletedCount.Inc()
											runIDsToDelete[shardID] = ""
											break
										}
									}
								}
							} else {
								request := generateRecordWorkflowExecutionStartedRequest(shardID, taskID)
								if runIDsToDelete[shardID] == "" {
									runIDsToDelete[shardID] = request.Execution.GetRunId()
								}
								for {
									// Emulate task processor which retries forever.
									err1 := visibilityManager.RecordWorkflowExecutionStarted(request)
									if err1 != nil {
										errCount.Inc()
									} else {
										break
									}
								}
							}

							// Emulate real load, not constant load. Add 2-4 seconds delay.
							// time.Sleep(time.Duration(rand.Intn(2000)+2000) * time.Millisecond)

							if exit {
								break
							}
						}
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
			},
		},

		{
			Name: "query",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "index",
					Required: true,
					Usage:    "index to query",
				},
				&cli.StringFlag{
					Name:  "type",
					Value: "list",
					Usage: "index to query",
				},
				&cli.IntFlag{
					Name:  "pf",
					Value: 200,
					Usage: "parallel factor",
				},
				&cli.IntFlag{
					Name:  "records",
					Value: 1_000_000,
					Usage: "records count",
				},
			},
			Action: func(c *cli.Context) error {
				cfg, err := config.LoadConfig(c.String("env"), "./config", "")
				if err != nil {
					return cli.Exit(fmt.Sprintf("unable to load configuration: %v", err), 1)
				}

				if c.IsSet("es-password") && c.String("es-password") != "" {
					cfg.Elasticsearch.Password = c.String("es-password")
				}

				indexName := c.String("index")
				visibilityManager, err := newVisibilityManager(cfg, indexName)
				if err != nil {
					return cli.Exit(fmt.Sprintf("unable to create Elasticsearch visibility manager: %v", err), 1)
				}

				start := time.Now()
				wg := &sync.WaitGroup{}

				parallelFactor := c.Int("pf")
				recordCount := c.Int("records")
				pageSize := 100

				taskIDs := rand.Perm(recordCount / parallelFactor)

				fmt.Printf("=== Input: %d recordCount, with parallel factor: %d\n", recordCount, parallelFactor)

				requestCounts := make([]int, parallelFactor)
				errCount := &atomic.Int32{}
				foundCount := &atomic.Int64{}
				wg.Add(parallelFactor)

				for shardID := 0; shardID < parallelFactor; shardID++ {
					go func(shardID int) {
						for taskID := range taskIDs {
							// request := &manager.ListWorkflowExecutionsRequestV2{
							// 	NamespaceID: namespaceID,
							// 	Query:       fmt.Sprintf(`CustomKeywordField="search-string-%d-%d"`, shardID, taskID),
							// 	PageSize: 1,
							// }
							_ = taskID
							request := &manager.ListWorkflowExecutionsRequestV2{
								NamespaceID: namespaceID,
								PageSize: pageSize,
							}

							var resp *manager.ListWorkflowExecutionsResponse
							var err error
							if c.String("type") == "list" {
								resp, err = visibilityManager.ListWorkflowExecutions(request)
							} else if c.String("type") == "scan" {
								resp, err = visibilityManager.ScanWorkflowExecutions(request)
							} else {
								panic("unknown type")
							}

							requestCounts[shardID]++

							if err != nil {
								errCount.Inc()
								fmt.Println(err)
							} else {
								foundCount.Add(int64(len(resp.Executions)))
							}
							if exit {
								break
							}
						}
						wg.Done()
					}(shardID)
				}

				wg.Wait()
				took := time.Since(start)
				requestCount := 0
				totalRPS := float64(0)
				for _, count := range requestCounts {
					requestCount += count
					totalRPS += float64(count)/took.Seconds()
				}

				fmt.Printf("=== Input: %d recordCount, with parallel factor: %d\n", recordCount, parallelFactor)
				fmt.Printf("=== Issued %d requests, with %d errors, %d results, took %v, avg RPS %f\n",
					requestCount,
					errCount.Load(),
					foundCount.Load(),
					took,
					totalRPS/float64(len(requestCounts)))

				return nil
			},
		},

		{
			Name: "delete",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "index",
					Required: true,
					Usage:    "index to delete or all",
				},
			},
			Action: func(c *cli.Context) error {
				cfg, err := config.LoadConfig("development", "./config", "")
				if err != nil {
					return cli.Exit(fmt.Sprintf("unable to load configuration: %v", err), 1)
				}

				rawClient, err := newRawClient(cfg.Elasticsearch)
				if err != nil {
					return cli.Exit(fmt.Sprintf("unable to create raw ES client: %v", err), 1)
				}

				var indexesToDelete []string
				if c.String("index") == "all" {
					resp, err := rawClient.CatIndices().Do(context.Background())
					if err != nil {
						return cli.Exit(fmt.Sprintf("unable to cat indexes: %v", err), 1)
					}
					for _, indexToDelete := range resp {
						indexesToDelete = append(indexesToDelete, indexToDelete.Index)
					}
				} else {
					indexesToDelete = append(indexesToDelete, c.String("index"))
				}

				for _, indexToDelete := range indexesToDelete {
					fmt.Println("Deleting index", indexToDelete)
					resp, err := rawClient.DeleteIndex(indexToDelete).Do(context.Background())
					if err != nil || !resp.Acknowledged {
						return cli.Exit(fmt.Sprintf("unable to delete index: %v", err), 1)
					}
				}

				return nil
			},
		},
	}
	return app
}

func getDocID(workflowID string, runID string) string {
	return fmt.Sprintf("%s%s%s", workflowID, "~", runID)
}

func generateRecordWorkflowExecutionStartedRequest(shardID int32, taskID int) *manager.RecordWorkflowExecutionStartedRequest {
	request := &manager.RecordWorkflowExecutionStartedRequest{
		VisibilityRequestBase: &manager.VisibilityRequestBase{
			NamespaceID: namespaceID,
			Execution: commonpb.WorkflowExecution{
				WorkflowId: workflowID,
				RunId:      uuid.New(),
			},
			WorkflowTypeName: randString(10),
			StartTime:        time.Now().UTC(),
			Status:           enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING,
			ExecutionTime:    time.Now().UTC(),
			TaskID:           int64(taskID),
			ShardID:          shardID,
			Memo: &commonpb.Memo{
				Fields: map[string]*commonpb.Payload{
					"memo1": payload.EncodeString("memo-value"),
				},
			},
			TaskQueue: "test-task-queue",
			SearchAttributes: &commonpb.SearchAttributes{
				IndexedFields: map[string]*commonpb.Payload{
					"CustomStringField":  payload.EncodeString(fmt.Sprintf("search string custom-%d-%d", shardID, taskID)),
					"CustomKeywordField": payload.EncodeString(fmt.Sprintf("search-string-%d-%d", shardID, taskID)),
				}},
		},
	}

	return request
}

func getVisibilityTaskKey(shardID int32, taskID int) string {
	return fmt.Sprintf("%d%s%d", shardID, "~", taskID)
}

func generateESDoc(request *manager.RecordWorkflowExecutionStartedRequest, visibilityTaskKey string, typeMap searchattribute.NameTypeMap) map[string]interface{} {
	doc := map[string]interface{}{
		searchattribute.VisibilityTaskKey: visibilityTaskKey,
		searchattribute.NamespaceID:       request.NamespaceID,
		searchattribute.WorkflowID:        request.Execution.WorkflowId,
		searchattribute.RunID:             request.Execution.RunId,
		searchattribute.WorkflowType:      request.WorkflowTypeName,
		searchattribute.StartTime:         request.StartTime,
		searchattribute.ExecutionTime:     request.ExecutionTime,
		searchattribute.ExecutionStatus:   request.Status.String(),
		searchattribute.TaskQueue:         request.TaskQueue,
	}

	if len(request.Memo.GetFields()) > 0 {
		data, err := request.Memo.Marshal()
		if err != nil {
			panic(err)
		}
		doc[searchattribute.Memo] = data
		doc[searchattribute.MemoEncoding] = enumspb.ENCODING_TYPE_PROTO3.String()
	}

	searchAttributes, err := searchattribute.Decode(request.SearchAttributes, &typeMap)
	if err != nil {
		panic("Unable to decode search attributes.")
	}
	for saName, saValue := range searchAttributes {
		doc[saName] = saValue
	}

	return doc
}

func createIndex(cfg *config.Config, indexSuffix string) (string, error) {
	esClient, err := client.NewIntegrationTestsClient(cfg.Elasticsearch.URL.String(), cfg.Elasticsearch.Version)
	if err != nil {
		return "", err
	}

	indexName := fmt.Sprintf("%s_%s", cfg.Elasticsearch.Indices[client.VisibilityAppName], indexSuffix)
	ctx := context.Background()
	ok, err := esClient.CreateIndex(ctx, indexName)
	if err != nil {
		return "", err
	}

	if !ok {
		return "", fmt.Errorf("CreateIndex request is not acknowledged")
	}

	return indexName, nil
}

func createIndexRaw(client *elastic.Client, numIndexes int) ([]string, error) {
	result := make([]string, numIndexes)

	for i := 0; i < numIndexes; i++ {
		indexName := fmt.Sprintf("%s-%s", "temporal-es-bench", randString(7))
		resp, err := client.CreateIndex(indexName).Do(context.Background())
		if err != nil || !resp.Acknowledged {
			fmt.Println(err)
			i--
			continue
		}
		fmt.Println("Created index", indexName)
		result[i] = indexName
	}

	return result, nil
}

func createSchemaTemplate(cfg *config.Config, schemaTemplateFile string) error {
	ctx := context.Background()
	template, err := ioutil.ReadFile(schemaTemplateFile)
	if err != nil {
		return err
	}
	templateName := fmt.Sprintf("%s_template", cfg.Elasticsearch.Indices[client.VisibilityAppName])
	fmt.Println("URL:", cfg.Elasticsearch.URL.String(), "template name:", templateName, "template:", strings.Replace(string(template)[:100], "\n", " ", -1))
	esClient, err := client.NewIntegrationTestsClient(cfg.Elasticsearch.URL.String(), cfg.Elasticsearch.Version)
	if err != nil {
		return err
	}
	ok, err := esClient.IndexPutTemplate(ctx, templateName, string(template))
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("IndexPutTemplate request is not acknowledged")
	}

	return nil
}

func newVisibilityManager(cfg *config.Config, indexName string) (manager.VisibilityManager, error) {
	logger := tlog.NewZapLogger(tlog.BuildZapLogger(cfg.Log))

	esClient, err := client.NewClient(cfg.Elasticsearch, nil, logger)
	if err != nil {
		return nil, err
	}

	metricsScope := cfg.Metrics.NewScope(logger)
	metricsClient := metrics.NewClient(metricsScope, metrics.History)

	return visibility.NewAdvancedManager(
		indexName,
		esClient,
		config.ProcessorConfig(cfg),
		searchattribute.NewTestProvider(),
		nil,
		dynamicconfig.GetIntPropertyFn(100000),
		dynamicconfig.GetIntPropertyFn(100000),
		metricsClient,
		logger)
}

func randString(length int) string {
	const lowercaseSet = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = lowercaseSet[rand.Int63()%int64(len(lowercaseSet))]
	}
	return string(b)
}

func newRawClient(cfg *client.Config) (*elastic.Client, error) {
	options := []elastic.ClientOptionFunc{
		elastic.SetURL(cfg.URL.String()),
		elastic.SetSniff(false),
		elastic.SetBasicAuth(cfg.Username, cfg.Password),

		// Disable health check so we don't block client creation (and thus temporal server startup)
		// if the ES instance happens to be down.
		elastic.SetHealthcheck(false),

		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(128*time.Millisecond, 513*time.Millisecond))),

		// critical to ensure decode of int64 won't lose precision
		elastic.SetDecoder(&elastic.NumberDecoder{}),
	}

	return elastic.NewClient(options...)
}

func deleteOldWorkflows(m manager.VisibilityManager) {
	listRequest := &manager.ListWorkflowExecutionsRequestV2{
		NamespaceID: namespaceID,
		Namespace:   namespaceID,
		PageSize:    1,
		// Query:         "ORDER BY RunId",
	}
	resp, err := m.ListWorkflowExecutions(listRequest)
	if err != nil {
		fmt.Printf("Unable to list workflow executions: %v: %v.\n", listRequest, err)
		return
	}
	deletedCount := 0
	for _, executionInfo := range resp.Executions {
		deleteRequest := &manager.VisibilityDeleteWorkflowExecutionRequest{
			NamespaceID: namespaceID,
			RunID:       executionInfo.Execution.RunId,
			WorkflowID:  executionInfo.Execution.WorkflowId,
			TaskID:      math.MaxInt,
		}
		err = m.DeleteWorkflowExecution(deleteRequest)
		if err != nil {
			fmt.Printf("Unable to delete workflow: %v: %v.\n", deleteRequest, err)
		} else {
			deletedCount++
			fmt.Println(deletedCount)
		}
	}
	fmt.Printf("Workflows deleted: %v.\n", deletedCount)
}
func deleteWorkflow(m manager.VisibilityManager, runID string) error {
	deleteRequest := &manager.VisibilityDeleteWorkflowExecutionRequest{
		NamespaceID: namespaceID,
		RunID:       runID,
		WorkflowID:  workflowID,
		TaskID:      math.MaxInt,
	}
	err := m.DeleteWorkflowExecution(deleteRequest)
	return err
}
