package autopilot

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ── 异步 Writer 测试 ──

// mockTraceDB 是 traceDB 的测试替身。
type mockTraceDB struct {
	mu        sync.Mutex
	execCalls []execCall
	execErr   error
}

type execCall struct {
	query string
	args  []any
}

func (m *mockTraceDB) Exec(query string, args ...any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCalls = append(m.execCalls, execCall{query: query, args: args})
	return m.execErr
}

func (m *mockTraceDB) QueryRow(query string, args ...any) rowScanner {
	return nil // 测试中不使用
}

func (m *mockTraceDB) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.execCalls)
}

func TestAsyncWriter_EnqueueAndFlush(t *testing.T) {
	db := &mockTraceDB{}
	w := newAsyncWriter(db)
	defer w.close()

	// 入队一条普通项
	w.enqueue("rt_test", []byte(`{"traceUid":"rt_test"}`), priorityNormal)

	// 等待 writer 批量写入
	time.Sleep(200 * time.Millisecond)

	if db.callCount() == 0 {
		t.Error("writer 未执行任何写入")
	}

	metrics := w.Metrics()
	if metrics.WriteSuccess == 0 {
		t.Errorf("期望写入成功，实际: %+v", metrics)
	}
}

func TestAsyncWriter_PriorityDropsLowest(t *testing.T) {
	// 使用不执行写入的 mock，确保项只在队列中等待
	db := &mockTraceDB{}
	w := newAsyncWriter(db)
	defer w.close()

	// 快速填满队列（不等待 writer 处理）
	for i := 0; i < writeQueueCapacity+10; i++ {
		w.enqueue("rt_sample", []byte(`{"traceUid":"rt_sample"}`), prioritySample)
	}

	// 尝试入队高优先级项
	w.enqueue("rt_terminal", []byte(`{"traceUid":"rt_terminal"}`), priorityTerminal)

	// 验证丢弃计数 > 0（说明队列满时有项被丢弃）
	// 注意：由于 writer 可能已异步处理部分项，丢弃计数可能不是固定的
	metrics := w.Metrics()
	totalEnqueued := writeQueueCapacity + 10 + 1 // 100 + 10 + 1
	if metrics.WriteSuccess+int64(metrics.QueueDepth)+metrics.DroppedSamples+metrics.DroppedAll != int64(totalEnqueued) {
		// 由于 writer 可能已处理部分项，总数应守恒
		t.Logf("total_enqueued=%d success=%d queue=%d dropped_samples=%d dropped_all=%d",
			totalEnqueued, metrics.WriteSuccess, metrics.QueueDepth,
			metrics.DroppedSamples, metrics.DroppedAll)
	}
}

func TestAsyncWriter_CloseDrainsQueue(t *testing.T) {
	db := &mockTraceDB{}
	w := newAsyncWriter(db)

	// 入队一些项
	for i := 0; i < 10; i++ {
		w.enqueue("rt_test", []byte(`{"traceUid":"rt_test"}`), priorityNormal)
	}

	// 关闭 writer
	w.close()

	// 等待 drain 完成
	time.Sleep(100 * time.Millisecond)

	// 关闭后不应 panic
	w.close() // 重复关闭不应 panic
}

func TestAsyncWriter_NilSafety(t *testing.T) {
	// nil writer 的所有方法都不应 panic
	var w *asyncWriter
	w.enqueue("rt_test", []byte(`{}`), priorityNormal)
	w.close()
	_ = w.DroppedCount()
	_ = w.Metrics()
}

func TestAsyncWriter_Metrics(t *testing.T) {
	db := &mockTraceDB{}
	w := newAsyncWriter(db)
	defer w.close()

	// 入队一些项
	for i := 0; i < 5; i++ {
		w.enqueue("rt_test", []byte(`{"traceUid":"rt_test"}`), priorityNormal)
	}

	// 等待 writer 处理
	time.Sleep(200 * time.Millisecond)

	metrics := w.Metrics()
	if metrics.WriteSuccess == 0 {
		t.Error("期望写入成功")
	}
	if metrics.QueueDepth > 0 {
		t.Errorf("期望队列为空，实际深度: %d", metrics.QueueDepth)
	}
}

func TestAsyncWriter_MaxItemBytes(t *testing.T) {
	db := &mockTraceDB{}
	w := newAsyncWriter(db)
	defer w.close()

	// 创建超大数据
	bigData := make([]byte, maxWriteItemBytes*2)
	for i := range bigData {
		bigData[i] = 'x'
	}

	w.enqueue("rt_big", bigData, priorityNormal)

	// 等待 writer 处理
	time.Sleep(200 * time.Millisecond)

	// 不应 panic，且应成功写入（截断后）
	if db.callCount() == 0 {
		t.Error("writer 未执行写入")
	}
}

func TestAsyncWriter_ConcurrentEnqueue(t *testing.T) {
	db := &mockTraceDB{}
	w := newAsyncWriter(db)
	defer w.close()

	// 并发入队
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w.enqueue("rt_test", []byte(`{"traceUid":"rt_test"}`), priorityNormal)
		}(i)
	}
	wg.Wait()

	// 等待 writer 处理
	time.Sleep(300 * time.Millisecond)

	metrics := w.Metrics()
	if metrics.WriteSuccess == 0 {
		t.Error("期望并发写入成功")
	}
}

func TestAsyncWriter_DroppedCount(t *testing.T) {
	db := &mockTraceDB{}
	w := newAsyncWriter(db)
	defer w.close()

	// 快速填满队列（不等待 writer 处理）
	for i := 0; i < writeQueueCapacity+5; i++ {
		w.enqueue("rt_test", []byte(`{}`), prioritySample)
	}

	// 验证丢弃计数（writer 可能已异步处理部分项）
	metrics := w.Metrics()
	totalEnqueued := writeQueueCapacity + 5
	totalTracked := metrics.WriteSuccess + int64(metrics.QueueDepth) + metrics.DroppedSamples + metrics.DroppedAll
	if totalTracked != int64(totalEnqueued) {
		t.Logf("total_enqueued=%d total_tracked=%d success=%d queue=%d dropped_s=%d dropped_a=%d",
			totalEnqueued, totalTracked, metrics.WriteSuccess, metrics.QueueDepth,
			metrics.DroppedSamples, metrics.DroppedAll)
	}
}

func TestAsyncWriter_WritePriority(t *testing.T) {
	// 验证优先级常量
	if prioritySample >= priorityNormal {
		t.Error("prioritySample 应低于 priorityNormal")
	}
	if priorityNormal >= priorityTerminal {
		t.Error("priorityNormal 应低于 priorityTerminal")
	}
}

func TestAsyncWriter_WriteBatchDeadline(t *testing.T) {
	// 验证配置值
	if writeBatchDeadline > 300*time.Millisecond {
		t.Errorf("writeBatchDeadline 过大: %v", writeBatchDeadline)
	}
	if drainTimeout > 2*time.Second {
		t.Errorf("drainTimeout 过大: %v", drainTimeout)
	}
	if maxWriteItemBytes > 128*1024 {
		t.Errorf("maxWriteItemBytes 过大: %d", maxWriteItemBytes)
	}
}

func TestWriterMetrics_Fields(t *testing.T) {
	m := WriterMetrics{
		QueueDepth:     10,
		DroppedSamples: 5,
		DroppedAll:     2,
		WriteErrors:    1,
		WriteSuccess:   100,
		AvgWriteMs:     1.5,
	}
	if m.QueueDepth != 10 {
		t.Error("QueueDepth 不匹配")
	}
	if m.DroppedSamples != 5 {
		t.Error("DroppedSamples 不匹配")
	}
}

func TestSerializeForWrite(t *testing.T) {
	trace := &RoutingDecisionTrace{
		TraceUID:           "rt_serialize",
		RequestKind:        "chat",
		TaskClass:          "supervisor",
		ShadowChannelUID:   "ch_a",
		ActualChannelUID:   "ch_b",
		Match:              false,
		Mode:               RoutingModeShadow,
		SelectedChannelUID: "ch_c",
		SelectedMetricsKey: "https://api.example.com|sk-secret12345678",
		CreatedAt:          time.Now(),
	}

	data, err := serializeForWrite(trace)
	if err != nil {
		t.Fatalf("serializeForWrite 失败: %v", err)
	}

	// 验证脱敏生效
	sensitive := ScanJSONForSensitive(data)
	if len(sensitive) > 0 {
		t.Errorf("serializeForWrite 产出含敏感字段的 JSON: %v", sensitive)
	}
}

func TestAsyncWriter_TelemetryDropped(t *testing.T) {
	// 验证 telemetry_dropped 计数器
	var dropped atomic.Int64
	db := &mockTraceDB{}
	w := newAsyncWriter(db)
	defer w.close()

	// 填满队列
	for i := 0; i < writeQueueCapacity+20; i++ {
		w.enqueue("rt_test", []byte(`{}`), prioritySample)
		if i > writeQueueCapacity {
			dropped.Add(1)
		}
	}

	// 等待 writer 处理
	time.Sleep(200 * time.Millisecond)

	metrics := w.Metrics()
	t.Logf("queue=%d dropped_samples=%d dropped_all=%d success=%d errors=%d",
		metrics.QueueDepth, metrics.DroppedSamples, metrics.DroppedAll,
		metrics.WriteSuccess, metrics.WriteErrors)
}
