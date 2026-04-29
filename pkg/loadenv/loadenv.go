// Пакет loadenv предоставляет простые утилиты для загрузки файлов в стиле
// .env в карту, установки переменных окружения глобально или заполнения
// структуры значениями из окружения через теги.
package loadenv

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// LoadEnvToMap читает файл env построчно и возвращает карту ключ/значение.
// Строки, начинающиеся с '#', игнорируются, а префикс 'export ' удаляется.
func LoadEnvToMap(filename string) (map[string]string, error) {
	envMap := make(map[string]string)

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		// Разбираем каждую строку на ключ и значение. Пустые строки и комментарии
		// пропускаются, а некорректные строки игнорируются.
		key, value, ok, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("failed to parse line: %w", err)
		}
		if !ok {
			continue
		}

		envMap[key] = value
	}

	return envMap, scanner.Err()
}

// LoadEnvGlobal загружает переменные окружения из файла и устанавливает их
// в окружение текущего процесса с помощью os.Setenv.
func LoadEnvGlobal(filename string) error {
	envMap, err := LoadEnvToMap(filename)
	if err != nil {
		return fmt.Errorf("failed to load env: %w", err)
	}

	for key, value := range envMap {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set env var %s: %w", key, err)
		}
	}
	return nil
}

// LoadEnvToStruct загружает переменные из файла и заполняет структуру,
// на которую указывает target. Поля структуры должны использовать теги `env`,
// можно также применять теги `required` и `default`.
func LoadEnvToStruct(filename string, target any) error {
	envMap, err := LoadEnvToMap(filename)
	if err != nil {
		return fmt.Errorf("failed to load env: %w", err)
	}
	return mapToStruct(envMap, target)
}

// parseLine разбирает одну строку env на ключ и значение. Она пропускает
// пустые строки, комментарии и некорректные записи, а также поддерживает
// необязательный префикс `export`.
func parseLine(line string) (key, value string, ok bool, err error) {
	line = strings.TrimPrefix(line, "\uFEFF")
	line = strings.TrimSpace(line)

	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}

	// Поддержка export: строка может начинаться с "export VAR=value".
	if after, ok := strings.CutPrefix(line, "export "); ok {
		line = strings.TrimSpace(after)
	}

	// Разделение по первому '=': значение может содержать дополнительные '='.
	before, after, found := strings.Cut(line, "=")
	if !found {
		return "", "", false, nil
	}

	key = strings.TrimSpace(before)
	if key == "" {
		return "", "", false, nil
	}
	rawValue := strings.TrimSpace(after)

	value, err = parseValue(rawValue)
	if err != nil {
		return "", "", false, err
	}

	return key, value, true, nil
}

// parseValue нормализует строковое значение из файла env. Она обрабатывает
// кавычные значения с escape-последовательностями и удаляет inline-комментарии
// для неквазифицированных значений.
func parseValue(val string) (string, error) {
	if val == "" {
		return "", nil
	}

	// Если значение в кавычках: допускаем escape-последовательности внутри
	// двойных кавычек и поддерживаем одинарные кавычки как простой литерал.
	if val[0] == '"' || val[0] == '\'' {
		quote := val[0]
		closingQuote := strings.LastIndex(val, string(quote))
		if closingQuote <= 0 {
			return "", fmt.Errorf("missing closing quote in value: %s", val)
		}
		val = val[1 : closingQuote+1]

		var result strings.Builder
		escaped := false

		for i := 0; i < len(val); i++ {
			c := val[i]

			if escaped {
				switch c {
				case 'n':
					result.WriteByte('\n')
				case 't':
					result.WriteByte('\t')
				case '\\':
					result.WriteByte('\\')
				default:
					result.WriteByte(c)
				}
				escaped = false
				continue
			}

			if c == '\\' && quote == '"' {
				escaped = true
				continue
			}

			if c == quote {
				break
			}

			result.WriteByte(c)
		}

		return result.String(), nil
	}

	// Без кавычек — убираем комментарий, начинающийся с '#'.
	if idx := strings.Index(val, "#"); idx != -1 {
		val = val[:idx]
	}

	return strings.TrimSpace(val), nil
}

// mapToStruct заполняет структуру, на которую указывает target, значениями из
// env. Поля структуры должны иметь тег `env`; необязательные теги `required`
// и `default` управляют поведением при отсутствии значения.
func mapToStruct(env map[string]string, target any) error {
	v := reflect.ValueOf(target)

	// Проверяем, что это указатель на struct
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be pointer to struct")
	}

	v = v.Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Берём имя из тега `env`. Тег `required:"true"` делает поле обязательным,
		// а `default` задаёт значение по умолчанию, если переменная не найдена.
		key := fieldType.Tag.Get("env")
		required := fieldType.Tag.Get("required")
		defaultVal := fieldType.Tag.Get("default")

		if key == "" {
			return fmt.Errorf("field %s: missing env tag", fieldType.Name)
		}
		val, ok := env[key]
		if !ok {
			if required == "true" {
				return fmt.Errorf("required env var not set: %s", key)
			}
			val = defaultVal
		}

		if !field.CanSet() {
			return fmt.Errorf("field %s: cannot set", fieldType.Name)
		}

		if err := setValue(field, val); err != nil {
			return fmt.Errorf("field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// setValue конвертирует строку в нужный тип и записывает её в поле структуры.
// Поддерживаются типы string, знаковые целые, bool, float и time.Duration.
func setValue(field reflect.Value, val string) error {
	switch field.Kind() {

	case reflect.String:
		field.SetString(val)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(n)

	case reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(val)
			if err != nil {
				return err
			}
			field.SetInt(int64(d))
			return nil
		}

		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(n)

	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		field.SetBool(b)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)

	default:
		return fmt.Errorf("unsupported type: %s", field.Kind())
	}

	return nil
}
