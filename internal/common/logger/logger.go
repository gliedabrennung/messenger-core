package logger

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/gliedabrennung/sedna/internal/common/reqid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var current atomic.Pointer[zap.SugaredLogger]

type zapLogger struct {
	level zap.AtomicLevel
}

func init() {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	l, err := cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	current.Store(l.Sugar())

	hlog.SetLogger(&zapLogger{level: cfg.Level})
}

func L() *zap.SugaredLogger {
	return current.Load()
}

func withCtx(ctx context.Context) *zap.SugaredLogger {
	l := current.Load()
	if id := reqid.FromContext(ctx); id != "" {
		return l.With("request_id", id)
	}
	return l
}

func (z *zapLogger) Trace(v ...any)  { current.Load().Debug(v...) }
func (z *zapLogger) Debug(v ...any)  { current.Load().Debug(v...) }
func (z *zapLogger) Info(v ...any)   { current.Load().Info(v...) }
func (z *zapLogger) Notice(v ...any) { current.Load().Info(v...) }
func (z *zapLogger) Warn(v ...any)   { current.Load().Warn(v...) }
func (z *zapLogger) Error(v ...any)  { current.Load().Error(v...) }
func (z *zapLogger) Fatal(v ...any)  { current.Load().Fatal(v...) }

func (z *zapLogger) Tracef(format string, v ...any)  { current.Load().Debugf(format, v...) }
func (z *zapLogger) Debugf(format string, v ...any)  { current.Load().Debugf(format, v...) }
func (z *zapLogger) Infof(format string, v ...any)   { current.Load().Infof(format, v...) }
func (z *zapLogger) Noticef(format string, v ...any) { current.Load().Infof(format, v...) }
func (z *zapLogger) Warnf(format string, v ...any)   { current.Load().Warnf(format, v...) }
func (z *zapLogger) Errorf(format string, v ...any)  { current.Load().Errorf(format, v...) }
func (z *zapLogger) Fatalf(format string, v ...any)  { current.Load().Fatalf(format, v...) }

func (z *zapLogger) CtxTracef(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Debugf(format, v...)
}

func (z *zapLogger) CtxDebugf(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Debugf(format, v...)
}

func (z *zapLogger) CtxInfof(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Infof(format, v...)
}

func (z *zapLogger) CtxNoticef(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Infof(format, v...)
}

func (z *zapLogger) CtxWarnf(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Warnf(format, v...)
}

func (z *zapLogger) CtxErrorf(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Errorf(format, v...)
}

func (z *zapLogger) CtxFatalf(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Fatalf(format, v...)
}

func (z *zapLogger) SetLevel(level hlog.Level) {
	var zapLevel zapcore.Level
	switch level {
	case hlog.LevelTrace, hlog.LevelDebug:
		zapLevel = zapcore.DebugLevel
	case hlog.LevelInfo, hlog.LevelNotice:
		zapLevel = zapcore.InfoLevel
	case hlog.LevelWarn:
		zapLevel = zapcore.WarnLevel
	case hlog.LevelError:
		zapLevel = zapcore.ErrorLevel
	case hlog.LevelFatal:
		zapLevel = zapcore.FatalLevel
	default:
		zapLevel = zapcore.InfoLevel
	}
	z.level.SetLevel(zapLevel)
}

func (z *zapLogger) SetOutput(writer io.Writer) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg.EncoderConfig),
		zapcore.AddSync(writer),
		z.level,
	)
	current.Store(zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar())
}

func Debug(v ...any) { current.Load().Debug(v...) }
func Info(v ...any)  { current.Load().Info(v...) }
func Warn(v ...any)  { current.Load().Warn(v...) }
func Error(v ...any) { current.Load().Error(v...) }
func Fatal(v ...any) { current.Load().Fatal(v...) }

func Debugf(format string, v ...any) { current.Load().Debugf(format, v...) }
func Infof(format string, v ...any)  { current.Load().Infof(format, v...) }
func Warnf(format string, v ...any)  { current.Load().Warnf(format, v...) }
func Errorf(format string, v ...any) { current.Load().Errorf(format, v...) }
func Fatalf(format string, v ...any) { current.Load().Fatalf(format, v...) }

func CtxDebugf(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Debugf(format, v...)
}

func CtxInfof(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Infof(format, v...)
}

func CtxWarnf(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Warnf(format, v...)
}

func CtxErrorf(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Errorf(format, v...)
}

func CtxFatalf(ctx context.Context, format string, v ...any) {
	withCtx(ctx).Fatalf(format, v...)
}
