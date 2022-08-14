package env

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
)

func MustLoad(cfg any) {
	if err := Load(cfg); err != nil {
		panic(err)
	}
}

func Load(cfg any) error {
	if err := load(cfg); err != nil {
		return fmt.Errorf("failed to load: %w", err)
	}
	return nil
}

func load(cfg any) error {
	t := reflect.TypeOf(cfg)
	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("invalid config - must be a struct pointer")
	}

	data := []string{}
	for i := 0; i < t.Elem().NumField(); i++ {
		f := t.Elem().Field(i)
		tdata, defined := f.Tag.Lookup("env")
		if !defined {
			continue
		}
		tg, err := parseTagData(tdata)
		if err != nil {
			return fmt.Errorf("failed to parse tag: %w", err)
		}

		v, err := tg.get()
		if err != nil {
			return fmt.Errorf("[%s] failed to get tag: %w", f.Name, err)
		}
		if v == "" {
			continue
		}
		k := f.Name
		jTag, defined := f.Tag.Lookup("json")
		if defined {
			k = strings.Split(jTag, ",")[0]
		}

		serialFmt := "%q:%s"
		if f.Type.Kind() == reflect.String {
			serialFmt = "%q:%q"
		}
		data = append(data, fmt.Sprintf(serialFmt, k, v))
	}
	return json.Unmarshal([]byte(fmt.Sprintf("{%s}", strings.Join(data, ","))), cfg)
}

type tag struct {
	key, dfault      string
	required, base64 bool
}

var isValidTagName = regexp.MustCompile(`^[A-Z]([A-Z\d_]*[A-Z\d])?$`).MatchString

func parseTagData(data string) (tag, error) {
	bits := strings.Split(data, ",")
	key, options := bits[0], bits[1:]
	if !isValidTagName(key) {
		return tag{}, fmt.Errorf("invalid key %q", key)
	}

	tg := tag{key: key}
	for _, opt := range options {
		if strings.HasPrefix(opt, "default") {
			defVal := strings.TrimPrefix("default=", opt)
			if defVal == opt {
				return tag{}, fmt.Errorf("invalid default option syntax; must follow pattern of 'default=<value>'")
			}
			tg.dfault = defVal
			continue
		}
		tg.required = tg.required || opt == "required"
		tg.base64 = tg.base64 || opt == "base64"
	}
	return tg, nil
}

func (t tag) get() (string, error) {
	raw, ok := os.LookupEnv(t.key)
	if t.required && !ok {
		return "", fmt.Errorf("%q is not set", t.key)
	}

	if raw == "" {
		if t.dfault == "" {
			return "", nil
		}
		raw = t.dfault
	}

	if !t.base64 {
		return raw, nil
	}

	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("failed to DecodeString: %w", err)
	}

	return string(b), nil
}
