# Валидация данных

MAX передаёт стартовые параметры мини-приложению с каждым запуском. Чтобы убедиться, что данные принадлежат реальным пользователям и не были изменены, валидируйте их

## Абстрактная логика проверки

Ниже описан общий алгоритм проверки объекта [`WebAppData`](bridge.md). Реализуйте его на любом языке программирования

Вводные данные: `BOT_TOKEN` — токен бота, `USER_URL` — URL, по которому открыто мини-приложение

1. Извлеките фрагмент из `USER_URL` — данные после символа `#`. В нём в формате `key=value` содержатся параметры платформы, среди которых — `WebAppData`. Убедитесь, что каждый параметр встречается ровно один раз
2. Преобразуйте значение `WebAppData` из `key=value` в `[['key', 'value']]`
3. Убедитесь, что ключ `hash` присутствует в параметрах ровно один раз. Сохраните оригинальный хеш и исключите его из массива
4. Примените URL-декодирование для всех значений (`value`), если ваша платформа не сделала этого автоматически
5. Отсортируйте массив по ключам в алфавитном порядке `a` → `z`
6. Сформируйте строку: `key1=value1\nkey2=value2`. Назовём её `launch_params`
7. Вычислите `secret_key` — ключ для проверки подписи. Для этого подпишите токен бота с помощью HMAC-SHA256, используя строку `WebAppData` в качестве ключа: `HMAC-SHA256('WebAppData', BOT_TOKEN)`
8. Вычислите собственную подпись: подпишите полученную на шаге 7 строку с помощью HMAC-SHA256, используя `secret_key`. В итоге это будет выглядеть так: `HMAC-SHA256(secret_key, launch_params)`
9. Преобразуйте подпись из шага 8 в hex-строку
10. Если hex-строка подписи равна значению параметра `hash` из оригинальной строки — данные подлинные

## Валидация данных на стороне мини-приложения

### Этап 1. Извлечение данных

При запуске мини-приложения клиентская часть получает закодированную строку для валидации данных через [`WebAppData`](bridge.md)

Извлеките параметры платформы из URL-фрагмента — данные после символа #. В параметре `WebAppData` содержатся данные для валидации. Эти данные также доступны через `window.WebApp.initData`

**Пример URL с параметрами**

```
https://example.com#WebAppData=chat%3D%257B%2522id%2522%253A12345%252C%2522type%2522%253A%2522DIALOG%2522%257D%26ip%3D192.168.0.1%26user%3D%257B%2522id%2522%253A67890%252C%2522first_name%2522%253A%2522Max%2522%252C%2522last_name%2522%253A%2522User%2522%252C%2522username%2522%253Anull%252C%2522language_code%2522%253A%2522ru%2522%252C%2522photo_url%2522%253Anull%257D%26query_id%3D4c0ab423-342b-4e45-aea4-2747dbc500cd%26auth_date%3D1771409719%26hash%3D<calculated_hash>&WebAppPlatform=web&WebAppVersion=26.2.8
```

### Этап 2. Подготовка данных

1. Разбейте значение `WebAppData` по символу `&` на пары `key=value`
2. Сохраните значение параметра `hash` и исключите его из дальнейшей обработки
3. Примените URL-декодирование ко всем значениям
4. Отсортируйте параметры по ключам в алфавитном порядке `a` → `z`
5. Сформируйте строку `launch_params`, объединив пары `key=value` с разделителем `\n` (0x0A)

**Пример подготовленных данных**

```
auth_date=1771409719\nchat={"id":12345,"type":"DIALOG"}\nip=192.168.0.1\nquery_id=4c0ab423-342b-4e45-aea4-2747dbc500cd\nuser={"id":67890,"first_name":"Max","last_name":"User","username":null,"language_code":"ru","photo_url":null}

// hash
'<calculated_hash>'
```

`auth_date` передаётся в формате Unix timestamp в секундах

### Этап 3. Создание ключа шифрования

Создайте ключ шифрования `secret_key` с помощью алгоритма HMAC-SHA256. Используйте строку `WebAppData` в качестве ключа и токен бота, который вы получили при [создании чат-бота](../chatbots/bots-create/manage.md) на платформе MAX для партнёров:

`HMAC_SHA256("WebAppData", Bot Token)`

```
HMAC_SHA256(
    "WebAppData",
    "YOUR_BOT_TOKEN" // Bot Token
)
```

### Этап 4. Вычисление подписи

Вычислите подпись `hash` стартовых параметров `initData`. Подпишите строку `launch_params` с помощью HMAC-SHA256, используя `secret_key` в качестве криптографического ключа. Преобразуйте результат в hex-строку:

`hex(HMAC_SHA256(secret_key, launch_params))`

### Этап 5. Сравнение результата

Сравните полученную hex-строку с оригинальным значением `hash` из `WebAppData`. Если подписи совпадают — данные подлинные. Если нет — данные были изменены

## Примеры кода

**TypeScript**

```typescript
// Вводные параметры
const BOT_TOKEN = 'YOUR_BOT_TOKEN';
const USER_LINK = 'https://example.com#WebAppData=...&WebAppPlatform=web&WebAppVersion=26.2.8';

// Извлекаем параметры платформы из фрагмента URL
const hashParams = new URLSearchParams(new URL(USER_LINK).hash.slice(1));
const appData: string = hashParams.get('WebAppData') || '';
const platform: string = hashParams.get('WebAppPlatform') || '';
const appVersion: string = hashParams.get('WebAppVersion') || '';

const validateAppData = async (appData: string, botToken: string): Promise<boolean> => {
    // Преобразуем appData из key1=value1&key2=value2 в [["key", "value"], ["key2", "value2"]]
    const params: string[][] = appData.split('&').map((x) => x.split('='));

    // Если hash встречается больше одного раза — прерываем проверку
    if (params.filter((x) => x[0] === 'hash').length !== 1) {
        return false;
    }

    // Сохраняем хеш, который пришёл вместе с параметрами
    const originalHash = params.find((x) => x[0] === 'hash');

    // Если хеш отсутствует — валидация невозможна
    if (!originalHash || typeof originalHash[1] !== 'string') {
        return false;
    }

    // Производим URL-декодирование значений параметров
    for (const param of params) {
        param[1] = decodeURIComponent(param[1]);
    }

    // Сортируем параметры по названию ключа a -> z
    params.sort((a, b) => a[0].localeCompare(b[0]));

    // Формируем строку для подписи с разделителем \n, исключаем hash
    const launchParams = params
        .filter((x) => x[0] !== 'hash')
        .map((x) => `${x[0]}=${x[1]}`)
        .join('\n');

    // Преобразуем строку для подписи и токен бота в массивы байтов
    const encoder = new TextEncoder();
    const botTokenBytes = encoder.encode(botToken);
    const launchParamsBytes = encoder.encode(launchParams);

    // Создаём secret_key: подписываем токен бота с помощью HMAC-SHA256,
    // используя строку "WebAppData" в качестве ключа
    const launchParamsKeyBytes = await crypto.subtle.sign(
        'HMAC',
        await crypto.subtle.importKey(
            'raw',
            encoder.encode('WebAppData'),
            {
                name: 'HMAC',
                hash: {
                    name: 'SHA-256',
                },
            },
            false,
            ['sign'],
        ),
        botTokenBytes,
    );

    // Создаём подпись параметров с помощью HMAC-SHA256, используя secret_key
    const signature = await crypto.subtle.sign(
        'HMAC',
        await crypto.subtle.importKey(
            'raw',
            launchParamsKeyBytes,
            {
                name: 'HMAC',
                hash: {
                    name: 'SHA-256',
                },
            },
            false,
            ['sign'],
        ),
        launchParamsBytes,
    );

    // Переводим подпись из массива байтов в hex-формат
    const hash = Array.from(new Uint8Array(signature))
        .map(b => ('00' + b.toString(16))
        .slice(-2))
        .join('');

    // Сравниваем с полученным хешем
    return hash === originalHash[1];
};

console.log(await validateAppData(appData, BOT_TOKEN));
```

**Python**

```python
import hmac
import hashlib
from urllib.parse import urlparse, parse_qsl, unquote
from operator import itemgetter

# Вводные параметры
BOT_TOKEN = 'YOUR_BOT_TOKEN'
USER_LINK = 'https://example.com#WebAppData=...&WebAppPlatform=web&WebAppVersion=26.2.8'

# Извлекаем параметры платформы из фрагмента URL
fragment_dict = dict(parse_qsl(urlparse(USER_LINK).fragment))
app_data = fragment_dict.get('WebAppData', '')
platform = fragment_dict.get('WebAppPlatform', '')
app_version = fragment_dict.get('WebAppVersion', '')

def validate_app_data(app_data: str, bot_token: str) -> bool:
    """
    Проверяет валидность данных мини-приложения с помощью HMAC-SHA256.
    """
    # Парсим строку данных (key1=value1&key2=value2...) в список кортежей
    # parse_qsl автоматически выполняет URL-декодирование значений
    params = list(dict(parse_qsl(app_data, keep_blank_values=True)).items())

    # Ищем параметр hash и удаляем его из списка параметров для проверки
    original_hash = next((value for key, value in params if key == 'hash'), None)

    # Если хеш отсутствует — валидация невозможна
    if not original_hash:
        return False

    # Формируем список параметров без hash, сортируем по ключу (a -> z)
    params_to_sign = sorted([(k, v) for k, v in params if k != 'hash'], key=itemgetter(0))

    # Формируем строку для подписи с разделителем \n, исключаем hash
    launch_params = '\n'.join(f'{k}={v}' for k, v in params_to_sign)

    # Создаём secret_key: подписываем токен бота с помощью HMAC-SHA256,
    # используя строку "WebAppData" в качестве ключа
    secret_key = hmac.new(
        key=b"WebAppData",
        msg=bot_token.encode('utf-8'),
        digestmod=hashlib.sha256
    ).digest()

    # Вычисляем подпись WebAppData с помощью HMAC-SHA256, используя secret_key
    hash = hmac.new(
        key=secret_key,
        msg=launch_params.encode('utf-8'),
        digestmod=hashlib.sha256
    ).hexdigest()

    # Сравниваем полученный хеш с пришедшим
    return hmac.compare_digest(hash, original_hash)

print(validate_app_data(app_data, BOT_TOKEN))
```

**Go**

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Вводные параметры
const BotToken = "YOUR_BOT_TOKEN"
const UserLink = "https://example.com#WebAppData=...&WebAppPlatform=web&WebAppVersion=26.2.8"

func main() {
	// Извлекаем фрагмент из URL (данные после #)
	parts := strings.SplitN(UserLink, "#", 2)
	if len(parts) < 2 {
		panic("Invalid URL: no hash fragment found")
	}
	fragment := parts[1]

	// Разбираем параметры фрагмента
	var appDataEncoded string
	fragmentParams := strings.Split(fragment, "&")
	for _, p := range fragmentParams {
        if strings.HasPrefix(p, "WebAppData=") {
            // Если значение уже было записано, значит это дубликат
            if appDataEncoded != "" {
                panic("Duplicate WebAppData parameter found")
            }
            appDataEncoded = strings.TrimPrefix(p, "WebAppData=")
        }
    }

    if appDataEncoded == "" {
        panic("WebAppData parameter not found")
    }

	// Декодируем значение WebAppData
	appData, err := url.QueryUnescape(appDataEncoded)
	if err != nil {
		panic(err)
	}

	// Запускаем проверку
	isValid, err := ValidateAppData(appData, BotToken)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Print(isValid)
}

type paramPair struct {
	key   string
	value string
}

func ValidateAppData(initData, botToken string) (bool, error) {
	// Разбиваем строку по "&" на пары ключ=значение
	pairs := strings.Split(initData, "&")
	var params []paramPair
	var originalHash string

	// Перебираем параметры
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		val := kv[1]

		// Сохраняем оригинальный хеш и исключаем его
		if key == "hash" {
			// Если originalHash уже заполнен, значит встретили второй такой ключ
			if originalHash != "" {
				return false, fmt.Errorf("duplicate hash parameter found")
			}
			originalHash = val
			continue
		}

		// Производим URL-декодирование
		decodedVal, err := url.QueryUnescape(val)
		if err != nil {
			return false, fmt.Errorf("failed to decode val: %v", err)
		}

		params = append(params, paramPair{key: key, value: decodedVal})
	}

	// Если hash не найден — валидация невозможна
	if originalHash == "" {
		return false, fmt.Errorf("hash not found in initData")
	}

	// Сортируем параметры по алфавиту ключа (a -> z)
	sort.Slice(params, func(i, j int) bool {
		return params[i].key < params[j].key
	})

	// Формируем строку launch_params
	var launchParamsList []string
	for _, p := range params {
		launchParamsList = append(launchParamsList, fmt.Sprintf("%s=%s", p.key, p.value))
	}
	launchParams := strings.Join(launchParamsList, "\n")

	// Создаём secret_key: подписываем токен бота с помощью HMAC-SHA256,
	// используя строку "WebAppData" в качестве ключа
	h1 := hmac.New(sha256.New, []byte("WebAppData"))
	h1.Write([]byte(botToken))
	secretKey := h1.Sum(nil)

	// Создаём подпись параметров с помощью HMAC-SHA256, используя secret_key
	h2 := hmac.New(sha256.New, secretKey)
	h2.Write([]byte(launchParams))
	signature := h2.Sum(nil)

	// Переводим в hex
	hash := hex.EncodeToString(signature)

	// Сравниваем
	return hmac.Equal([]byte(hash), []byte(originalHash)), nil
}
```

**Java**

```java
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.net.URI;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.*;
import java.util.stream.Collectors;

public class MaxSignValidator {
    public static void main(String[] args) {
        // Вводные параметры
        String BOT_TOKEN = "YOUR_BOT_TOKEN";
        String USER_LINK = "https://example.com#WebAppData=...&WebAppPlatform=web&WebAppVersion=26.2.8";

        try {
            // Извлекаем параметры платформы из фрагмента URL
            String fragment = URI.create(USER_LINK).getRawFragment();
            Map<String, String> fragmentParams = parseQueryString(fragment);
            String appData = fragmentParams.getOrDefault("WebAppData", "");
            String platform = fragmentParams.getOrDefault("WebAppPlatform", "");
            String appVersion = fragmentParams.getOrDefault("WebAppVersion", "");

            // Проверяем валидность WebAppData
            System.out.println(validateAppData(appData, BOT_TOKEN));
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    public static boolean validateAppData(String appData, String botToken)
            throws NoSuchAlgorithmException, InvalidKeyException {
        if (appData == null || appData.isEmpty()) {
            return false;
        }

        // Преобразуем appData из key1=value1&key2=value2 в [["key", "value"], ...]
        List<String[]> params = new ArrayList<>();
        String[] pairs = appData.split("&");
        for (String pair : pairs) {
            int idx = pair.indexOf("=");
            if (idx > 0) {
                String key = pair.substring(0, idx);
                String value = URLDecoder.decode(
                    pair.substring(idx + 1), StandardCharsets.UTF_8
                );
                params.add(new String[]{key, value});
            }
        }

        // Находим оригинальный хеш
        String originalHash = null;
        for (String[] param : params) {
            if ("hash".equals(param[0])) {
                originalHash = param[1];
                break;
            }
        }

        // Если хеша нет — валидация невозможна
        if (originalHash == null) {
            return false;
        }

        // Сортируем параметры по алфавиту (a -> z)
        params.sort(Comparator.comparing(a -> a[0]));

        // Формируем строку для подписи с разделителем \n, исключаем hash
        String launchParams = params.stream()
                .filter(x -> !"hash".equals(x[0]))
                .map(x -> x[0] + "=" + x[1])
                .collect(Collectors.joining("\n"));

        // Создаём secret_key: подписываем токен бота с помощью HMAC-SHA256,
        // используя строку "WebAppData" в качестве ключа
        byte[] secretKeyBytes = hmacSha256(
            "WebAppData".getBytes(StandardCharsets.UTF_8),
            botToken.getBytes(StandardCharsets.UTF_8)
        );

        // Создаём подпись параметров с помощью HMAC-SHA256, используя secret_key
        byte[] signature = hmacSha256(
            secretKeyBytes,
            launchParams.getBytes(StandardCharsets.UTF_8)
        );

        // Переводим подпись в hex-формат
        String hash = bytesToHex(signature);

        // Сравниваем с полученным хешем
        return hash.equalsIgnoreCase(originalHash);
    }

    private static byte[] hmacSha256(byte[] key, byte[] message)
            throws NoSuchAlgorithmException, InvalidKeyException {
        Mac mac = Mac.getInstance("HmacSHA256");
        SecretKeySpec secretKeySpec = new SecretKeySpec(key, "HmacSHA256");
        mac.init(secretKeySpec);
        return mac.doFinal(message);
    }

    private static String bytesToHex(byte[] bytes) {
        StringBuilder sb = new StringBuilder();
        for (byte b : bytes) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }

    private static Map<String, String> parseQueryString(String query) {
        Map<String, String> queryPairs = new LinkedHashMap<>();
        if (query == null || query.isEmpty()) return queryPairs;

        String[] pairs = query.split("&");
        for (String pair : pairs) {
            int idx = pair.indexOf("=");

            String key = (idx > 0)
                ? URLDecoder.decode(pair.substring(0, idx), StandardCharsets.UTF_8)
                : URLDecoder.decode(pair, StandardCharsets.UTF_8);

            String value = (idx > 0 && pair.length() > idx + 1)
                ? URLDecoder.decode(pair.substring(idx + 1), StandardCharsets.UTF_8)
                : "";

            if (queryPairs.containsKey(key)) {
                throw new IllegalArgumentException("Duplicate parameter found: " + key);
            }

            queryPairs.put(key, value);
        }

        return queryPairs;
    }
}
```

ℹ️ Если у вас возникли вопросы, [посмотрите раздел с ответами](../../help/index.md)

---

Источник: <https://dev.max.ru/docs/webapps/validation>
