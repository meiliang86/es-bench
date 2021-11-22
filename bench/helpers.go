package bench

import (
	"math/rand"
	"time"

	"github.com/olivere/elastic/v7"
	"github.com/pborman/uuid"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/server/common/payload"
	"go.temporal.io/server/common/persistence/visibility/manager"
	"go.temporal.io/server/common/persistence/visibility/store/elasticsearch/client"
)

var lowercaseSet = []rune("abcdefghijklmnopqrstuvwxyz")

func randString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = lowercaseSet[rand.Intn(len(lowercaseSet))]
	}
	return string(b)
}

func generateRecordWorkflowExecutionStartedRequest(shardID int32, taskID int) *manager.RecordWorkflowExecutionStartedRequest {
	request := &manager.RecordWorkflowExecutionStartedRequest{
		VisibilityRequestBase: &manager.VisibilityRequestBase{
			NamespaceID: namespaceID,
			Execution: commonpb.WorkflowExecution{
				WorkflowId: randString(10),
				RunId:      uuid.New(),
			},
			WorkflowTypeName: "test-workflow",
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
			//SearchAttributes: &commonpb.SearchAttributes{
			//	IndexedFields: map[string]*commonpb.Payload{
			//		"CustomStringField":  payload.EncodeString(fmt.Sprintf("search string custom-%d-%d", shardID, taskID)),
			//		"CustomKeywordField": payload.EncodeString(fmt.Sprintf("search-string-%d-%d", shardID, taskID)),
			//	}},
		},
	}

	return request
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