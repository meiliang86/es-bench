package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/temporalio/es-bench/bench"
	"github.com/temporalio/es-bench/config"
	"github.com/urfave/cli/v2"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/server/common/persistence/visibility/manager"
	"go.temporal.io/server/common/searchattribute"
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
	done := make(chan bool)
	go func() {
		select {
		case <-ctrlC:
			close(done)
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
			Name: "ingest",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "index-name",
					Aliases: []string{"in"},
					Value: "temporal_es_bench_test",
					Usage: "index name",
				},
				&cli.StringFlag{
					Name:  "namespace-id",
					Aliases: []string{"ns"},
					Value: "default",
					Usage: "namespace id",
				},
				&cli.IntFlag{
					Name:  "total-records",
					Aliases: []string{"tr"},
					Value: 100,
					Usage: "records count",
				},
				&cli.IntFlag{
					Name:  "parallel-factor",
					Aliases: []string{"pf"},
					Value: 10,
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
				cfg := loadConfig(c)
				indexName := c.String("index-name")
				b := bench.NewBench(cfg, done, indexName)
				recordCount := c.Int("total-records")
				parallelFactor := c.Int("parallel-factor")
				namespaceID := c.String("namespace-id")
				return b.IngestData(recordCount, parallelFactor, namespaceID)
			},
		},
		{
			Name: "query",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "index-name",
					Aliases: []string{"in"},
					Value: "temporal_es_bench_test",
					Usage: "index name",
				},
				&cli.StringFlag{
					Name:  "namespace-id",
					Aliases: []string{"ns"},
					Value: "default",
					Usage: "namespace id",
				},
				&cli.IntFlag{
					Name:  "total-records",
					Aliases: []string{"tr"},
					Value: 100,
					Usage: "records count",
				},
				&cli.StringFlag{
					Name:  "type",
					Value: "list",
					Usage: "index to query",
				},
				&cli.IntFlag{
					Name:  "parallel-factor",
					Aliases: []string{"pf"},
					Value: 1,
					Usage: "parallel factor",
				},
				&cli.IntFlag{
					Name:  "time-range-secs",
					Aliases: []string{"trs"},
					Value: 60,
					Usage: "time filter range in seconds",
				},
			},
			Action: func(c *cli.Context) error {
				cfg := loadConfig(c)
				indexName := c.String("index-name")
				namespaceID := c.String("namespace-id")
				b := bench.NewBench(cfg, done, indexName)
				recordCount := c.Int("total-records")
				parallelFactor := c.Int("parallel-factor")
				queryType := c.String("type")
				timeRange := time.Duration(c.Int("time-range-secs")) * time.Second
				b.QueryData(recordCount, parallelFactor, queryType, timeRange, namespaceID)
				<-done
				return nil
			},
		},
		{
			Name: "index",
			Subcommands: []*cli.Command{
				{
					Name: "create-template",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "template-path",
							Value: "./schema/index_template.json",
							Usage: "Path to the template json file.",
						},
					},
					Action: func(c *cli.Context) error {
						cfg := loadConfig(c)
						templatePath := c.String("template-path")
						b := bench.NewBench(cfg, done, templatePath)
						err := b.CreateSchemaTemplate(templatePath)
						if err != nil {
							fmt.Printf("Failed to create index template: %v\n", err)
						} else {
							fmt.Println("Successfully created index template.")
						}
						return err
					},
				},
				{
					Name: "create",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "index-name",
							Required: true,
							Usage:    "index name",
						},
					},
					Action: func(c *cli.Context) error {
						cfg := loadConfig(c)
						indexName := c.String("index-name")
						b := bench.NewBench(cfg, done, indexName)
						return b.CreateIndex(indexName)
					},
				},
				{
					Name: "delete",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "index-name",
							Required: true,
							Usage:    "index name",
						},
					},
					Action: func(c *cli.Context) error {
						cfg := loadConfig(c)
						indexName := c.String("index-name")
						b := bench.NewBench(cfg, done, indexName)
						return b.DeleteIndex(indexName)
					},
				},
			},
		},
		{
			Name: "idle",
			Action: func(context *cli.Context) error {
				<-done
				return nil
			},
		},
	}
	return app
}

func loadConfig(c *cli.Context) *config.Config {
	cfg, err := config.LoadConfig(c.String("env"), "./config", "")
	if err != nil {
		panic(fmt.Sprintf("unable to load configuration: %v", err))
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

	return cfg
}



func getDocID(workflowID string, runID string) string {
	return fmt.Sprintf("%s%s%s", workflowID, "~", runID)
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

//func createIndexRaw(client *elastic.Client, numIndexes int) ([]string, error) {
//	result := make([]string, numIndexes)
//
//	for i := 0; i < numIndexes; i++ {
//		indexName := fmt.Sprintf("%s-%s", "temporal-es-bench", randString(7))
//		resp, err := client.CreateIndex(indexName).Do(context.Background())
//		if err != nil || !resp.Acknowledged {
//			fmt.Println(err)
//			i--
//			continue
//		}
//		fmt.Println("Created index", indexName)
//		result[i] = indexName
//	}
//
//	return result, nil
//}

//
//func deleteOldWorkflows(m manager.VisibilityManager) {
//	listRequest := &manager.ListWorkflowExecutionsRequestV2{
//		NamespaceID: namespaceID,
//		Namespace:   namespaceID,
//		PageSize:    1,
//		// Query:         "ORDER BY RunId",
//	}
//	resp, err := m.ListWorkflowExecutions(listRequest)
//	if err != nil {
//		fmt.Printf("Unable to list workflow executions: %v: %v.\n", listRequest, err)
//		return
//	}
//	deletedCount := 0
//	for _, executionInfo := range resp.Executions {
//		deleteRequest := &manager.VisibilityDeleteWorkflowExecutionRequest{
//			NamespaceID: namespaceID,
//			RunID:       executionInfo.Execution.RunId,
//			WorkflowID:  executionInfo.Execution.WorkflowId,
//			TaskID:      math.MaxInt,
//		}
//		err = m.DeleteWorkflowExecution(deleteRequest)
//		if err != nil {
//			fmt.Printf("Unable to delete workflow: %v: %v.\n", deleteRequest, err)
//		} else {
//			deletedCount++
//			fmt.Println(deletedCount)
//		}
//	}
//	fmt.Printf("Workflows deleted: %v.\n", deletedCount)
//}
