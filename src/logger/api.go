package logger

type Logger interface {
	Debug(msg string, attrs ...Attr)
	Info(msg string, attrs ...Attr)
	Warn(msg string, attrs ...Attr)
	Error(msg string, err error, attrs ...Attr)
	Fatal(msg string, err error, attrs ...Attr)

	With(attrs ...Attr) Logger
	Named(name string) Logger
	Sync() error
}

type Attr struct {
	key   string
	value any
	kind  attrKind
}

type attrKind uint8

const (
	kindString attrKind = iota
	kindInt
	kindInt64
	kindBool
	kindAny
)

func String(key, value string) Attr {
	return Attr{
		key:   key,
		value: value,
		kind:  kindString,
	}
}

func Int(key string, value int) Attr {
	return Attr{
		key:   key,
		value: value,
		kind:  kindInt,
	}
}

func Int64(key string, value int64) Attr {
	return Attr{
		key:   key,
		value: value,
		kind:  kindInt64,
	}
}

func Bool(key string, value bool) Attr {
	return Attr{
		key:   key,
		value: value,
		kind:  kindBool,
	}
}

func Any(key string, value any) Attr {
	return Attr{
		key:   key,
		value: value,
		kind:  kindAny,
	}
}
