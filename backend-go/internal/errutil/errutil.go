// Package errutil 提供显式忽略错误的辅助函数。
package errutil

// IgnoreDeferred 调用延迟执行的清理函数，并有意忽略其错误。
// 传入方法值可保持直接 defer 调用时的接收者求值时机。
func IgnoreDeferred(cleanup func() error) {
	_ = cleanup()
}
