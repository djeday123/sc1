package rod

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// NewsItem представляет структуру данных для новостной статьи
type NewsItem struct {
	Title       string
	Description string
	URL         string
	Source      string
	Timestamp   time.Time
	Query       string // Поисковый запрос
}

// ScrapeGoogleNews выполняет поиск новостей в Google с помощью Rod
func ScrapeGoogleNews(query string) ([]NewsItem, error) {
	var newsItems []NewsItem

	maxPagesToCheck := 3  // Максимум 3 страницы результатов
	targetNewsCount := 20 // Целевое количество настоящих новостей

	// Настраиваем прокси, если он задан
	proxyServer := os.Getenv("PROXY_SERVER")

	proxyUser := os.Getenv("PROXY_USER")
	proxyPass := os.Getenv("PROXY_PASS")

	// Режим отладки - показывать браузер или нет
	headless := os.Getenv("HEADLESS") != "false"

	// Настройка запуска браузера
	launcherObj := launcher.New()

	// Добавляем флаги для запуска Chrome
	launcherObj = launcherObj.
		Headless(headless).
		NoSandbox(true).
		Set("disable-web-security", "true").
		Set("disable-features", "IsolateOrigins,site-per-process")

	// Настройка прокси, если он задан
	if proxyServer != "" {
		log.Printf("Setting up proxy: %s", proxyServer)
		launcherObj = launcherObj.Set("proxy-server", proxyServer).Set("host-resolver-rules", "MAP * ~NOTFOUND , EXCLUDE localhost").Set("proxy-bypass-list", "<-loopback>")
	}

	// Запускаем браузер и получаем URL для подключения
	browserURL := launcherObj.MustLaunch()

	// Создаем браузер
	browser := rod.New().
		ControlURL(browserURL).
		Timeout(120 * time.Second)

	// Подключаемся к браузеру
	err := browser.Connect()
	if err != nil {
		return nil, fmt.Errorf("error connecting to browser: %w", err)
	}
	defer browser.Close()

	// Настройка прокси-аутентификации, если требуется
	if proxyServer != "" && proxyUser != "" && proxyPass != "" {
		log.Printf("Configuring proxy authentication for user: %s", proxyUser)
		browser.MustHandleAuth(proxyUser, proxyPass)
	}

	// Добавляем случайные задержки для имитации человеческого поведения
	randomDelay := func(min, max int) {
		delay := min + rand.Intn(max-min)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Создаем новую страницу
	page := browser.MustPage("")

	// Настройка User-Agent
	err = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	})
	if err != nil {
		return nil, fmt.Errorf("error setting user-agent: %w", err)
	}

	// Устанавливаем размер окна
	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  1920,
		Height: 1080,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting viewport: %w", err)
	}

	// Маскировка автоматизации
	_, err = page.Eval(`
		(function() {
			if (navigator.__proto__) {
				delete navigator.__proto__.webdriver;
			}
			return true;
		})()
	`)

	//if err != nil {
	//	log.Printf("Warning: Could not execute anti-detection script: %v", err)
	//}

	// Переходим на Google
	log.Println("Navigating to Google...")
	randomDelay(500, 1500)

	err = page.Navigate("https://www.google.com")
	if err != nil {
		return nil, fmt.Errorf("error navigating to Google: %w", err)
	}

	// Ждем загрузки страницы
	err = page.WaitLoad()
	if err != nil {
		return nil, fmt.Errorf("error waiting for page to load: %w", err)
	}

	// Принимаем cookie, если появится диалог
	log.Println("Checking for cookie consent dialog...")
	cookieSelectors := []string{
		"button:has-text('Accept all')",
		"button:has-text('Aceptar todo')",
		"button:has-text('Tout accepter')",
		"button:has-text('Alle akzeptieren')",
		"button:has-text('Accetta tutto')",
		"button:has-text('Принять все')",
		"button:has-text('Tümünü kabul et')",
		"button:has-text('Aceitar tudo')",
		"button:has-text('Accepteer alles')",
		"button:has-text('Hamısını qəbul et')",
		"button:has-text('同意所有')",
		"button:has-text('すべて同意')",
	}

	for _, selector := range cookieSelectors {
		cookieBtn, err := page.Timeout(2 * time.Second).Element(selector)
		if err == nil && cookieBtn != nil {
			log.Printf("Cookie consent dialog found with selector: %s", selector)
			randomDelay(300, 800)
			err = cookieBtn.Click(proto.InputMouseButtonLeft, 1) // Fixed: added click count parameter
			if err == nil {
				log.Println("Clicked on cookie accept button, waiting for page to stabilize...")
				page.WaitLoad()
				break
			} else {
				log.Printf("Warning: Could not click cookie button: %v", err)
			}
		}
	}

	// Ожидание для имитации человеческого поведения
	randomDelay(1000, 2000)

	// Находим поле поиска
	log.Printf("Entering search query: %s", query)
	searchInput, err := page.Element("input[name='q'], textarea[name='q']")
	if err != nil {
		return nil, fmt.Errorf("error finding search input: %w", err)
	}

	// Вводим текст с человеческой скоростью
	for _, char := range query {
		err = searchInput.Input(string(char))
		if err != nil {
			return nil, fmt.Errorf("error typing search query: %w", err)
		}
		randomDelay(50, 150)
	}

	randomDelay(500, 1200) // Пауза перед отправкой

	// Отправляем форму нажатием Enter
	//err = searchInput.Input(proto.InputKey{Key: "Enter"}) // Fixed: replaced Press with Input
	//err = searchInput.Press("Enter") // Simulate pressing the Enter key
	err = searchInput.Input("\r") // Символ возврата каретки - эквивалент Enter
	if err != nil {
		return nil, fmt.Errorf("error submitting search query: %w", err)
	}

	// Ждем загрузки результатов
	err = page.WaitLoad()
	if err != nil {
		return nil, fmt.Errorf("error waiting for search results: %w", err)
	}

	// Проверяем наличие антибот-защиты
	captchaRes, err := page.Eval(`
		!!document.querySelector('iframe[src*="recaptcha"]') || 
		!!document.querySelector('#captcha') ||
		!!document.querySelector('.g-recaptcha') ||
		document.title.includes('unusual traffic') ||
		document.body.innerText.includes('unusual traffic') ||
		document.body.innerText.includes('security check')
	`)

	if err == nil && captchaRes.Value.Bool() { // Fixed: use Value.Bool() instead of direct assertion
		log.Println("Anti-bot challenge detected! Please solve it manually.")
		log.Println("Waiting 10 seconds for manual intervention...")
		time.Sleep(10 * time.Second)
	} else {
		randomDelay(2000, 4000)
	}

	// Пытаемся найти и кликнуть на раздел новостей
	log.Println("Searching for News section...")
	newsSelectors := []string{
		`a:has-text("Новости")`,
		`a:has-text("News")`,
		`a:has-text("Noticias")`,
		`a:has-text("Xəbərlər")`,
		`a:has-text("Actualités")`,
		`a:has-text("Haberler")`,
		`a[href*="tbm=nws"]`,
	}

	newsLinkFound := false
	for _, selector := range newsSelectors {
		newsLink, err := page.Timeout(2 * time.Second).Element(selector)
		if err == nil && newsLink != nil {
			log.Printf("News section found with selector: %s", selector)
			randomDelay(500, 1000)
			err = newsLink.Click(proto.InputMouseButtonLeft, 1) // Fixed: added click count parameter
			if err == nil {
				log.Println("Clicked on news section, waiting for results...")
				newsLinkFound = true
				break
			} else {
				log.Printf("Error clicking news section: %v", err)
			}
		}
	}

	// Если не нашли раздел новостей, переходим напрямую
	if !newsLinkFound {
		log.Println("Could not find news section link, trying direct URL approach...")
		newsUrl := fmt.Sprintf("https://www.google.com/search?q=%s&tbm=nws", url.QueryEscape(query))
		err = page.Navigate(newsUrl)
		if err != nil {
			return nil, fmt.Errorf("error navigating directly to news: %w", err)
		}

		err = page.WaitLoad()
		if err != nil {
			return nil, fmt.Errorf("error waiting for news page to load: %w", err)
		}
	}

	// Создаем контекст с таймаутом для извлечения данных
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Для нескольких страниц результатов
	for pageNum := 1; pageNum <= maxPagesToCheck; pageNum++ {
		log.Printf("Processing page %d of %d", pageNum, maxPagesToCheck)

		// Проверяем наличие заголовков новостей
		headingsRes, err := page.Eval(`
			const headings = document.querySelectorAll('div[role="heading"]');
			return headings.length > 0 ? headings.length : 0;
		`)

		if err != nil || headingsRes.Value.Int() == 0 { // Fixed: use Value.Int() instead of direct assertion
			log.Println("No headings found with div[role='heading'], trying alternative selectors...")

			// Альтернативные селекторы для заголовков
			headlineSelectors := []string{".n0jPhd", ".MBeuO", "article h3", "h3", ".DY5T1d"}
			headlinesFound := false

			for _, selector := range headlineSelectors {
				log.Printf("Trying selector: %s", selector)

				// Fixed: pass selector as parameter in JS
				elementsRes, err := page.Eval(fmt.Sprintf(`
					const elements = document.querySelectorAll("%s");
					return elements.length;
				`, selector))

				if err == nil && elementsRes.Value.Int() > 0 { // Fixed: use Value.Int() instead of direct assertion
					log.Printf("Found %v elements with selector: %s", elementsRes.Value.Int(), selector)
					headlinesFound = true

					// Извлекаем новости с текущим селектором
					err = extractNewsWithSelector(ctx, page, selector, query, &newsItems, targetNewsCount)
					if err != nil {
						log.Printf("Error extracting news with selector %s: %v", selector, err)
						continue
					}
					break
				}
			}

			if !headlinesFound && pageNum == 1 {
				// Если это первая страница и ничего не найдено, делаем скриншот и выходим
				screenshotPath := "no_headlines.png"
				buf, err := page.Screenshot(false, &proto.PageCaptureScreenshot{}) // Fixed: added correct params
				if err != nil {
					log.Printf("Error taking screenshot: %v", err)
				} else {
					err = os.WriteFile(screenshotPath, buf, 0644)
					if err != nil {
						log.Printf("Error saving screenshot to file: %v", err)
					} else {
						log.Printf("Screenshot saved to %s", screenshotPath)
					}
				}
				return nil, fmt.Errorf("could not find any news headlines")
			} else if !headlinesFound {
				// Для последующих страниц просто выходим из цикла
				log.Println("No more headlines found on additional pages")
				break
			}
		} else {
			log.Printf("Found %v heading elements with div[role='heading']", headingsRes.Value.Int())
			// Извлекаем новости с основным селектором
			err = extractNewsWithSelector(ctx, page, "div[role='heading']", query, &newsItems, targetNewsCount)
			if err != nil {
				log.Printf("Error extracting news: %v", err)
			}
		}

		// Проверяем, достигли ли целевого количества новостей
		if len(newsItems) >= targetNewsCount {
			log.Printf("Reached target count of %d news items", targetNewsCount)
			break
		}

		// Если это не последняя страница и нужно больше новостей
		if pageNum < maxPagesToCheck && len(newsItems) < targetNewsCount {
			log.Println("Looking for next page button...")

			// Пытаемся найти кнопку следующей страницы
			nextButtonFound := false
			nextButtonSelectors := []string{
				`[aria-label="Next page"]`,
				`[aria-label="Siguiente página"]`,
				`a:has-text("Next")`,
				`a:has-text("Siguiente")`,
			}

			for _, selector := range nextButtonSelectors {
				nextButton, err := page.Timeout(2 * time.Second).Element(selector)
				if err == nil && nextButton != nil {
					log.Printf("Found next page button with selector: %s", selector)
					randomDelay(500, 1000)

					err = nextButton.Click(proto.InputMouseButtonLeft, 1) // Fixed: added click count parameter
					if err == nil {
						log.Println("Clicked next page button, waiting for page to load...")
						page.WaitLoad()
						nextButtonFound = true
						break
					} else {
						log.Printf("Error clicking next page button: %v", err)
					}
				}
			}

			if !nextButtonFound {
				log.Println("No more pages available")
				break
			}
		} else {
			// Достигли лимита страниц или количества новостей
			break
		}
	}

	log.Printf("Scraping completed. Found %d news items.", len(newsItems))

	// Делаем скриншот результата
	screenshotPath := "final_results.png"
	buf, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
	if err != nil {
		log.Printf("Error taking final screenshot: %v", err)
	} else {
		// Сохраняем буфер в файл
		err = os.WriteFile(screenshotPath, buf, 0644)
		if err != nil {
			log.Printf("Error saving screenshot to file: %v", err)
		} else {
			log.Printf("Saved final screenshot to %s", screenshotPath)
		}
	}
	return newsItems, nil
}

// extractNewsWithSelector извлекает новости с указанным селектором
func extractNewsWithSelector(ctx context.Context, page *rod.Page, selector, query string, newsItems *[]NewsItem, targetNewsCount int) error {
	// Получаем все элементы с заголовками
	headlines, err := page.Elements(selector)
	if err != nil {
		return fmt.Errorf("error selecting headlines with selector %s: %w", selector, err)
	}

	log.Printf("Found %d headlines with selector %s, extracting details...", len(headlines), selector)

	for i, headline := range headlines {
		select {
		case <-ctx.Done():
			log.Println("Context timeout reached, returning partial results")
			return ctx.Err()
		default:
			log.Printf("Processing news item %d/%d...", i+1, len(headlines))

			var newsItem NewsItem
			newsItem.Timestamp = time.Now()
			newsItem.Query = query

			// Получаем заголовок
			title, err := headline.Text()
			if err != nil {
				log.Printf("Error getting title for item %d: %v", i+1, err)
				continue
			}
			newsItem.Title = strings.TrimSpace(title)
			log.Printf("Got title: %s", newsItem.Title)

			// Проверяем, является ли элемент новостью
			isNewsItemRes, err := page.Eval(fmt.Sprintf(`
				function checkNewsItem(el) {
					// Проверяем, есть ли у элемента или его родителя ссылка
					const anchor = el.closest('a[href]');
					if (!anchor) return false;
					
					// Проверяем, что это не элемент управления
					const text = el.textContent.toLowerCase();
					const isFilterItem = text.includes('elige') || 
										text.includes('fecha') || 
										text.includes('intervalo') ||
										text.includes('opinión');
					
					return !isFilterItem && anchor.getAttribute('href').length > 10;
				}
				checkNewsItem(document.querySelectorAll("%s")[%d]);
			`, selector, i))

			// Пропускаем элементы, которые не являются новостями
			if err == nil && isNewsItemRes != nil && !isNewsItemRes.Value.Bool() {
				log.Printf("Skipping non-news item: %s", newsItem.Title)
				continue
			}

			// Получаем URL новости
			urlValueRes, err := page.Eval(fmt.Sprintf(`
				function getNewsUrl(el) {
					const anchor = el.closest('a[href]');
					if (!anchor) return "";
					
					// Проверяем, это редирект Google или прямая ссылка
					if (anchor.href.includes('/url?')) {
						const url = new URL(anchor.href);
						// Параметр q в редиректе Google содержит оригинальный URL
						return url.searchParams.get('q') || url.searchParams.get('url');
					}
					return anchor.href;
				}
				getNewsUrl(document.querySelectorAll("%s")[%d]);
			`, selector, i))

			if err == nil && urlValueRes != nil && urlValueRes.Value.String() != "" {
				newsItem.URL = urlValueRes.Value.String()
				log.Printf("Got URL: %s", newsItem.URL)
			}

			// Получаем описание
			descTextRes, err := page.Eval(fmt.Sprintf(`
				function getNewsDescription(el) {
					const container = el.closest('a[href]');
					if (!container) return "";
					
					// Ищем описание в div с классом GI74Re
					const desc = container.querySelector('.GI74Re');
					if (desc) return desc.textContent;
					
					// Если не нашли, ищем в следующем элементе
					const nextSibling = container.nextElementSibling;
					if (nextSibling) return nextSibling.textContent;
					
					return "";
				}
				getNewsDescription(document.querySelectorAll("%s")[%d]);
			`, selector, i))

			if err == nil && descTextRes != nil && descTextRes.Value.String() != "" {
				newsItem.Description = strings.TrimSpace(descTextRes.Value.String())
				log.Printf("Got description: %s", newsItem.Description)
			}

			// Получаем источник
			sourceTextRes, err := page.Eval(fmt.Sprintf(`
				function getNewsSource(el) {
					const container = el.closest('a[href]');
					if (!container) return "";
					
					// Ищем источник в span внутри div с классом MgUUmf
					const source = container.querySelector('.MgUUmf span');
					if (source) return source.textContent;
					
					// Также ищем в элементе с классом QyR1Ze
					const altSource = container.querySelector('.QyR1Ze');
					if (altSource) return altSource.textContent;
					
					return "";
				}
				getNewsSource(document.querySelectorAll("%s")[%d]);
			`, selector, i))

			if err == nil && sourceTextRes != nil && sourceTextRes.Value.String() != "" {
				newsItem.Source = strings.TrimSpace(sourceTextRes.Value.String())
				log.Printf("Got source: %s", newsItem.Source)
			}

			// Добавляем новость в список
			*newsItems = append(*newsItems, newsItem)

			// Если достигли нужного количества, завершаем
			if len(*newsItems) >= targetNewsCount {
				log.Printf("Reached target count of %d news items", targetNewsCount)
				return nil
			}
		}
	}

	return nil
}

// CheckIPAddress проверяет текущий IP-адрес
func CheckIPAddress() (string, error) {
	// Настройка запуска браузера
	launcherObj := launcher.New().
		Headless(true).
		NoSandbox(true)

	// Настройка прокси, если он задан
	proxyServer := os.Getenv("PROXY_SERVER")
	proxyUser := os.Getenv("PROXY_USER")
	proxyPass := os.Getenv("PROXY_PASS")

	if proxyServer != "" {
		log.Printf("Setting up proxy for IP check: %s", proxyServer)
		launcherObj = launcherObj.Set("proxy-server", proxyServer)
	}

	// Запускаем браузер и получаем URL для подключения
	browserURL := launcherObj.MustLaunch()

	// Создаем браузер
	browser := rod.New().
		ControlURL(browserURL).
		Timeout(30 * time.Second)

	// Подключаемся к браузеру
	err := browser.Connect()
	if err != nil {
		return "", fmt.Errorf("error connecting to browser: %w", err)
	}
	defer browser.Close()

	// Настройка прокси-аутентификации, если требуется
	if proxyServer != "" && proxyUser != "" && proxyPass != "" {
		browser.MustHandleAuth(proxyUser, proxyPass)
	}

	// Создаем новую страницу
	page := browser.MustPage("")

	// Переходим на сервис, показывающий IP
	log.Println("Checking IP address via ipinfo.io...")
	err = page.Navigate("https://ipinfo.io/json")
	if err != nil {
		return "", fmt.Errorf("error navigating to IP check service: %w", err)
	}

	// Ждем загрузки страницы
	err = page.WaitLoad()
	if err != nil {
		return "", fmt.Errorf("error waiting for IP check page to load: %w", err)
	}

	// Извлекаем информацию об IP
	ipInfoRes, err := page.Eval(`
		try {
			const data = JSON.parse(document.body.innerText);
			return {
				ip: data.ip,
				location: data.city + ", " + data.region + ", " + data.country,
				org: data.org
			};
		} catch (e) {
			return { error: e.message };
		}
	`)
	if err != nil {
		return "", fmt.Errorf("error extracting IP details: %w", err)
	}

	return fmt.Sprintf("%v", ipInfoRes.Value), nil
}
