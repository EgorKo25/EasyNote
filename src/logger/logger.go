package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/EgorKo25/EasyNote/src/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var errUnsupportedLoggerOutput = errors.New("unsupported logger output")

var globalLogger Logger = &zapLogger{
	log: zap.NewNop(),
}

func Initialize(app config.AppConfig, cfg config.Logger) error {
	cores := make([]zapcore.Core, 0, len(cfg.Cores))

	for _, coreCfg := range cfg.Cores {
		core, err := newCore(coreCfg)
		if err != nil {
			return err
		}

		cores = append(cores, core)
	}

	log := zap.New(
		zapcore.NewTee(cores...),
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.Fields(
			zap.String("service", app.Name),
			zap.String("environment", app.Environment),
		),
	)

	globalLogger = &zapLogger{
		log: log,
	}

	return nil
}

func Get(name string) Logger {
	return globalLogger.Named(name)
}

type zapLogger struct {
	log *zap.Logger
}

func newCore(cfg config.LoggerCore) (zapcore.Core, error) {
	level := zapcore.InfoLevel
	if err := level.Set(cfg.Level); err != nil {
		return nil, fmt.Errorf("set logger level %q: %w", cfg.Level, err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = timeEncoder(cfg.TimeFormat)
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder

	writer, err := newOutput(cfg)
	if err != nil {
		return nil, err
	}

	return zapcore.NewCore(
		newEncoder(cfg.Encoding, encoderCfg),
		writer,
		level,
	), nil
}

func newEncoder(encoding string, cfg zapcore.EncoderConfig) zapcore.Encoder {
	if encoding == "console" {
		return zapcore.NewConsoleEncoder(cfg)
	}

	return zapcore.NewJSONEncoder(cfg)
}

func newOutput(cfg config.LoggerCore) (zapcore.WriteSyncer, error) {
	switch cfg.Output {
	case "stdout":
		return zapcore.Lock(zapcore.AddSync(os.Stdout)), nil

	case "stderr":
		return zapcore.Lock(zapcore.AddSync(os.Stderr)), nil

	case "file":
		return newFileWriteSyncer(cfg.Path)

	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedLoggerOutput, cfg.Output)
	}
}

func newFileWriteSyncer(path string) (zapcore.WriteSyncer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("creating a directory for the log file %q: %w", path, err)
	}

	file, err := os.OpenFile(filepath.Clean(path), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	return zapcore.AddSync(file), nil
}

func timeEncoder(format string) zapcore.TimeEncoder {
	if format == "local" {
		return zapcore.ISO8601TimeEncoder
	}

	return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format(time.RFC3339Nano))
	}
}

func (l *zapLogger) Debug(msg string, attrs ...Attr) {
	l.log.Debug(msg, attrsToZapFields(attrs)...)
}

func (l *zapLogger) Info(msg string, attrs ...Attr) {
	l.log.Info(msg, attrsToZapFields(attrs)...)
}

func (l *zapLogger) Warn(msg string, attrs ...Attr) {
	l.log.Warn(msg, attrsToZapFields(attrs)...)
}

func (l *zapLogger) Error(msg string, err error, attrs ...Attr) {
	fields := attrsToZapFields(attrs)
	fields = append(fields, zap.Error(err))

	l.log.Error(msg, fields...)
}

func (l *zapLogger) Fatal(msg string, err error, attrs ...Attr) {
	fields := attrsToZapFields(attrs)
	fields = append(fields, zap.Error(err))

	l.log.Fatal(msg, fields...)
}

func (l *zapLogger) With(attrs ...Attr) Logger {
	return &zapLogger{
		log: l.log.With(attrsToZapFields(attrs)...),
	}
}

func (l *zapLogger) Named(name string) Logger {
	return &zapLogger{
		log: l.log.Named(name),
	}
}

func (l *zapLogger) Sync() error {
	if err := l.log.Sync(); err != nil {
		return fmt.Errorf("sync logger: %w", err)
	}

	return nil
}

func attrsToZapFields(attrs []Attr) []zap.Field {
	fields := make([]zap.Field, 0, len(attrs))

	for _, attr := range attrs {
		fields = append(fields, attrToZapField(attr))
	}

	return fields
}

//nolint:forcetypeassert // an error when casting a type is not important and is unlikely, so the linter is disabled
func attrToZapField(attr Attr) zap.Field {
	switch attr.kind {
	case kindString:
		return zap.String(attr.key, attr.value.(string))
	case kindInt:
		return zap.Int(attr.key, attr.value.(int))
	case kindInt64:
		return zap.Int64(attr.key, attr.value.(int64))
	case kindBool:
		return zap.Bool(attr.key, attr.value.(bool))
	case kindAny:
		return zap.Any(attr.key, attr.value)
	}

	return zap.Any(attr.key, attr.value)
}
