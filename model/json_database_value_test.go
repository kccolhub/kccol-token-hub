package model

import (
	"context"
	"database/sql/driver"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func TestJSONDatabaseValuersReturnText(t *testing.T) {
	tests := []struct {
		name     string
		valuer   driver.Valuer
		wantJSON string
	}{
		{
			name:     "channel info",
			valuer:   ChannelInfo{IsMultiKey: true, MultiKeySize: 2},
			wantJSON: `{"is_multi_key":true,"multi_key_size":2,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":""}`,
		},
		{
			name:     "task properties",
			valuer:   Properties{Input: "prompt", UpstreamModelName: "upstream-model"},
			wantJSON: `{"input":"prompt","upstream_model_name":"upstream-model"}`,
		},
		{
			name:     "task private data",
			valuer:   TaskPrivateData{UpstreamTaskID: "upstream-task"},
			wantJSON: `{"upstream_task_id":"upstream-task"}`,
		},
		{
			name:     "prefill group items",
			valuer:   JSONValue(`["gpt-4o","gpt-5"]`),
			wantJSON: `["gpt-4o","gpt-5"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.valuer.Value()
			require.NoError(t, err)
			textValue, ok := value.(string)
			require.True(t, ok, "JSON database values must be strings, got %T", value)
			assert.JSONEq(t, tt.wantJSON, textValue)
		})
	}
}

func TestEmptyJSONDatabaseValuersRemainNull(t *testing.T) {
	values := []driver.Valuer{
		Properties{},
		TaskPrivateData{},
		JSONValue(nil),
	}

	for _, valuer := range values {
		value, err := valuer.Value()
		require.NoError(t, err)
		assert.Nil(t, value)
	}
}

func TestTaskDataSerializerReturnsText(t *testing.T) {
	taskSchema, err := schema.Parse(&Task{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	dataField := taskSchema.LookUpField("Data")
	require.NotNil(t, dataField)

	task := &Task{}
	task.SetData(map[string]any{"status": "complete"})
	value, zero := dataField.ValueOf(context.Background(), reflect.ValueOf(task))
	require.False(t, zero)

	valuer, ok := value.(driver.Valuer)
	require.True(t, ok, "GORM JSON serializer must provide a driver.Valuer, got %T", value)
	databaseValue, err := valuer.Value()
	require.NoError(t, err)
	textValue, ok := databaseValue.(string)
	require.True(t, ok, "serialized task data must be text, got %T", databaseValue)
	assert.JSONEq(t, `{"status":"complete"}`, textValue)
}

func TestJSONDatabaseScannersAcceptTextAndBytes(t *testing.T) {
	var channelInfo ChannelInfo
	require.NoError(t, channelInfo.Scan(`{"is_multi_key":true,"multi_key_size":2}`))
	assert.True(t, channelInfo.IsMultiKey)
	assert.Equal(t, 2, channelInfo.MultiKeySize)

	var properties Properties
	require.NoError(t, properties.Scan([]byte(`{"input":"prompt"}`)))
	assert.Equal(t, "prompt", properties.Input)

	var privateData TaskPrivateData
	require.NoError(t, privateData.Scan(`{"upstream_task_id":"upstream-task"}`))
	assert.Equal(t, "upstream-task", privateData.UpstreamTaskID)

	var items JSONValue
	require.NoError(t, items.Scan(`["gpt-4o","gpt-5"]`))
	assert.JSONEq(t, `["gpt-4o","gpt-5"]`, string(items))
}
