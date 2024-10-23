package clickhousetracesexporter

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

func (w *SpanWriter) writeIndexBatchV2(ctx context.Context, batchSpans []*SpanV2) error {
	var statement driver.Batch
	var err error

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
	}()
	statement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, "distributed_signoz_index_v3"), driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("Could not prepare batch for index table: ", zap.Error(err))
		return err
	}
	for _, span := range batchSpans {
		err = statement.Append(
			span.TsBucketStart,
			span.FingerPrint,
			time.Unix(0, int64(span.StartTimeUnixNano)),
			span.Id,
			span.TraceId,
			span.SpanId,
			span.TraceState,
			span.ParentSpanId,
			span.Flags,
			span.Name,
			span.Kind,
			span.SpanKind,
			span.DurationNano,
			span.StatusCode,
			span.StatusMessage,
			span.StatusCodeString,
			span.AttributeString,
			span.AttributesNumber,
			span.AttributesBool,
			span.ResourcesString,
			span.Events,

			span.ServiceName,

			// composite attributes
			span.ResponseStatusCode,
			span.ExternalHttpUrl,
			span.HttpUrl,
			span.ExternalHttpMethod,
			span.HttpMethod,
			span.HttpHost,
			span.DBName,
			span.DBOperation,
			span.HasError,
			span.IsRemote,

			// single attributes
			span.HttpRoute,
			span.MsgSystem,
			span.MsgOperation,
			span.DBSystem,
			span.RPCSystem,
			span.RPCService,
			span.RPCMethod,
			span.PeerService,

			span.References,
		)
		if err != nil {
			w.logger.Error("Could not append span to batch: ", zap.Any("span", span), zap.Error(err))
			return err
		}
	}

	start := time.Now()

	err = statement.Send()

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
		tag.Upsert(tableKey, w.indexTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
	return err
}

func (w *SpanWriter) writeErrorBatchV2(ctx context.Context, batchSpans []*SpanV2) error {
	var statement driver.Batch
	var err error

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
	}()
	statement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.errorTable), driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("Could not prepare batch for error table: ", zap.Error(err))
		return err
	}

	for _, span := range batchSpans {
		if span.ErrorEvent.Name == "" {
			continue
		}
		err = statement.Append(
			time.Unix(0, int64(span.ErrorEvent.TimeUnixNano)),
			span.ErrorID,
			span.ErrorGroupID,
			span.TraceId,
			span.SpanId,
			span.ServiceName,
			span.ErrorEvent.AttributeMap["exception.type"],
			span.ErrorEvent.AttributeMap["exception.message"],
			span.ErrorEvent.AttributeMap["exception.stacktrace"],
			stringToBool(span.ErrorEvent.AttributeMap["exception.escaped"]),
			span.ResourcesString,
		)
		if err != nil {
			w.logger.Error("Could not append span to batch: ", zap.Any("span", span), zap.Error(err))
			return err
		}
	}

	start := time.Now()

	err = statement.Send()

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
		tag.Upsert(tableKey, w.errorTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
	return err
}

func (w *SpanWriter) writeTagBatchV2(ctx context.Context, batchSpans []*SpanV2) error {
	var tagKeyStatement driver.Batch
	var tagStatement driver.Batch
	var err error

	defer func() {
		if tagKeyStatement != nil {
			_ = tagKeyStatement.Abort()
		}
		if tagStatement != nil {
			_ = tagStatement.Abort()
		}
	}()
	tagStatement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeTable), driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("Could not prepare batch for span attributes table due to error: ", zap.Error(err))
		return err
	}
	tagKeyStatement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeKeyTable), driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("Could not prepare batch for span attributes key table due to error: ", zap.Error(err))
		return err
	}
	// create map of span attributes of key, tagType, dataType and isColumn to avoid duplicates in batch
	mapOfSpanAttributeKeys := make(map[string]struct{})

	// create map of span attributes of key, tagType, dataType, isColumn and value to avoid duplicates in batch
	mapOfSpanAttributeValues := make(map[string]struct{})

	for _, span := range batchSpans {
		for _, spanAttribute := range span.SpanAttributes {

			// form a map key of span attribute key, tagType, dataType, isColumn and value
			mapOfSpanAttributeValueKey := spanAttribute.Key + spanAttribute.TagType + spanAttribute.DataType + strconv.FormatBool(spanAttribute.IsColumn) + spanAttribute.StringValue + strconv.FormatFloat(spanAttribute.NumberValue, 'f', -1, 64)

			// check if mapOfSpanAttributeValueKey already exists in map
			_, ok := mapOfSpanAttributeValues[mapOfSpanAttributeValueKey]
			if ok {
				continue
			}
			// add mapOfSpanAttributeValueKey to map
			mapOfSpanAttributeValues[mapOfSpanAttributeValueKey] = struct{}{}

			// form a map key of span attribute key, tagType, dataType and isColumn
			mapOfSpanAttributeKey := spanAttribute.Key + spanAttribute.TagType + spanAttribute.DataType + strconv.FormatBool(spanAttribute.IsColumn)

			// check if mapOfSpanAttributeKey already exists in map
			_, ok = mapOfSpanAttributeKeys[mapOfSpanAttributeKey]
			if !ok {
				err = tagKeyStatement.Append(
					spanAttribute.Key,
					spanAttribute.TagType,
					spanAttribute.DataType,
					spanAttribute.IsColumn,
				)
				if err != nil {
					w.logger.Error("Could not append span to tagKey Statement to batch due to error: ", zap.Error(err), zap.Any("span", span))
					return err
				}
			}
			// add mapOfSpanAttributeKey to map
			mapOfSpanAttributeKeys[mapOfSpanAttributeKey] = struct{}{}

			if spanAttribute.DataType == "string" {
				err = tagStatement.Append(
					time.Unix(0, int64(span.StartTimeUnixNano)),
					spanAttribute.Key,
					spanAttribute.TagType,
					spanAttribute.DataType,
					spanAttribute.StringValue,
					nil,
					spanAttribute.IsColumn,
				)
			} else if spanAttribute.DataType == "float64" {
				err = tagStatement.Append(
					time.Unix(0, int64(span.StartTimeUnixNano)),
					spanAttribute.Key,
					spanAttribute.TagType,
					spanAttribute.DataType,
					nil,
					spanAttribute.NumberValue,
					spanAttribute.IsColumn,
				)
			} else if spanAttribute.DataType == "bool" {
				err = tagStatement.Append(
					time.Unix(0, int64(span.StartTimeUnixNano)),
					spanAttribute.Key,
					spanAttribute.TagType,
					spanAttribute.DataType,
					nil,
					nil,
					spanAttribute.IsColumn,
				)
			}
			if err != nil {
				w.logger.Error("Could not append span to tag Statement batch due to error: ", zap.Error(err), zap.Any("span", span))
				return err
			}
		}
	}

	tagStart := time.Now()
	err = tagStatement.Send()
	stats.RecordWithTags(ctx,
		[]tag.Mutator{
			tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
			tag.Upsert(tableKey, w.attributeTable),
		},
		writeLatencyMillis.M(int64(time.Since(tagStart).Milliseconds())),
	)
	if err != nil {
		w.logger.Error("Could not write to span attributes table due to error: ", zap.Error(err))
		return err
	}

	tagKeyStart := time.Now()
	err = tagKeyStatement.Send()
	stats.RecordWithTags(ctx,
		[]tag.Mutator{
			tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
			tag.Upsert(tableKey, w.attributeKeyTable),
		},
		writeLatencyMillis.M(int64(time.Since(tagKeyStart).Milliseconds())),
	)
	if err != nil {
		w.logger.Error("Could not write to span attributes key table due to error: ", zap.Error(err))
		return err
	}

	return err
}

// WriteBatchOfSpans writes the encoded batch of spans
func (w *SpanWriter) WriteBatchOfSpansV2(ctx context.Context, batch []*SpanV2, metrics map[string]usage.Metric) error {
	var wg sync.WaitGroup
	var err error

	if w.indexTable != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = w.writeIndexBatchV2(ctx, batch)
			if err != nil {
				w.logger.Error("Could not write a batch of spans to index table: ", zap.Error(err))
			}
		}()
	}

	if w.errorTable != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = w.writeErrorBatchV2(ctx, batch)
			if err != nil {
				w.logger.Error("Could not write a batch of spans to error table: ", zap.Error(err))
			}
		}()
	}

	if w.attributeTable != "" && w.attributeKeyTable != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = w.writeTagBatchV2(ctx, batch)
			if err != nil {
				w.logger.Error("Could not write a batch of spans to tag/tagKey tables: ", zap.Error(err))
			}
		}()
	}
	wg.Wait()

	for k, v := range metrics {
		stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, w.exporterId.String())}, ExporterSigNozSentSpans.M(int64(v.Count)), ExporterSigNozSentSpansBytes.M(int64(v.Size)))
	}

	return err
}

func (w *SpanWriter) WriteResourcesV2(ctx context.Context, resourcesSeen map[int64]map[string]string) error {
	var insertResourcesStmtV2 driver.Batch

	defer func() {
		if insertResourcesStmtV2 != nil {
			_ = insertResourcesStmtV2.Abort()
		}
	}()

	insertResourcesStmtV2, err := w.db.PrepareBatch(
		ctx,
		fmt.Sprintf("INSERT into %s.%s", w.traceDatabase, "distributed_traces_v3_resource"),
		driver.WithReleaseConnection(),
	)
	if err != nil {
		return fmt.Errorf("couldn't PrepareBatch for inserting resource fingerprints :%w", err)
	}

	for bucketTs, resources := range resourcesSeen {
		for resourceLabels, fingerprint := range resources {
			insertResourcesStmtV2.Append(
				resourceLabels,
				fingerprint,
				bucketTs,
			)
		}
	}
	err = insertResourcesStmtV2.Send()
	if err != nil {
		return fmt.Errorf("couldn't send resource fingerprints :%w", err)
	}
	return nil
}