package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// NewsItem представляет структуру данных для новостной статьи
type NewsItem struct {
	Title       string
	Description string
	URL         string
	Source      string
	Timestamp   time.Time
	Query       string // Добавляем поле для запроса
}

// ScrapeGoogleNews выполняет поиск новостей в Google с использованием Chromedp
func ScrapeGoogleNews(query string) ([]NewsItem, error) {
	var newsItems []NewsItem

	maxPagesToCheck := 3  // Максимум 3 страницы результатов
	targetNewsCount := 20 // Целевое количество настоящих новостей

	// Настройка для отладки - показывать браузер или нет
	headless := os.Getenv("HEADLESS") != "false"

	// Настройка прокси
	proxyServer := os.Getenv("PROXY_SERVER")
	proxyUser := os.Getenv("PROXY_USER")
	proxyPass := os.Getenv("PROXY_PASS")

	// Инициализируем rand с текущим временем
	rand.Seed(time.Now().UnixNano())

	// Базовые опции Chromedp
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(1920, 1080),
	)

	// Добавление прокси, если он указан
	if proxyServer != "" {
		log.Printf("Setting up proxy: %s", proxyServer)
		opts = append(opts, chromedp.ProxyServer(proxyServer))

		// Если у прокси есть аутентификация
		if proxyUser != "" && proxyPass != "" {
			log.Printf("Proxy authentication configured: %s", proxyUser)
		}
	}

	// Создаем контекст с настройками
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// Создаем контекст с таймаутом для всей операции
	ctx, cancel := context.WithTimeout(allocCtx, 10*time.Minute)
	defer cancel()

	// Создаем контекст браузера
	ctx, cancel = chromedp.NewContext(
		ctx,
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// Настраиваем обработчики событий для логирования
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if e.Type == network.ResourceTypeDocument {
				log.Printf("Navigating to: %s", e.Request.URL)
			}
		case *network.EventResponseReceived:
			if e.Response.Status >= 400 {
				log.Printf("Error response: %d for %s", e.Response.Status, e.Response.URL)
			}
		}
	})

	// Функция для случайной задержки (имитация человека)
	randomDelay := func(min, max int) {
		delay := min + rand.Intn(max-min)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Выполняем действия в браузере
	var titleNodes []*cdp.Node

	log.Printf("Proxy server: %+v", proxyServer)

	// Если proxyServer является структурой или объектом, можно использовать %+v для подробного вывода
	// log.Printf("Proxy server details: %+v", proxyServer)

	err := chromedp.Run(ctx,
		// Маскировка автоматизации
		chromedp.Evaluate(`
			Object.defineProperty(navigator, 'webdriver', {
				get: () => false,
			});
			
			// Маскировка автоматизации
			if (navigator.__proto__) {
				delete navigator.__proto__.webdriver;
			}
			
			// Добавляем плагины для маскировки
			Object.defineProperty(navigator, 'plugins', {
				get: () => [1, 2, 3, 4, 5],
			});
		`, nil),

		// Переходим на Google
		chromedp.Navigate("https://www.google.com"),
		chromedp.WaitReady("body"),

		// Эмуляция действий пользователя: небольшая задержка
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Navigated to Google, waiting for page to stabilize...")
			randomDelay(1000, 2000)
			return nil
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("error navigating to Google: %w", err)
	}

	// Проверяем наличие диалога согласия на использование cookie
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Checking for cookie consent dialog...")
			return nil
		}),
		chromedp.Evaluate(`
			function checkAndAcceptCookies() {
				const selectors = [
					'button:has-text("Accept all")', 
					'button:has-text("Aceptar todo")', 
					'button:has-text("Tout accepter")',
					'button:has-text("Alle akzeptieren")',
					'button:has-text("Accetta tutto")',
					'button:has-text("Принять все")',
					'button:has-text("Tümünü kabul et")',
					'button:has-text("Aceitar tudo")',
					'button:has-text("Accepteer alles")',
					'button:has-text("Hamısını qəbul et")',
					'button:has-text("同意所有")',
					'button:has-text("すべて同意")'
				];
				
				for (const selector of selectors) {
					try {
						const btn = document.querySelector(selector);
						if (btn) {
							console.log("Found cookie button, clicking...");
							btn.click();
							return true;
						}
					} catch (e) {
						console.error("Error checking selector:", e);
					}
				}
				return false;
			}
			return checkAndAcceptCookies();
		`, nil),

		chromedp.ActionFunc(func(ctx context.Context) error {
			randomDelay(1000, 2000) // Задержка после возможного принятия cookie
			return nil
		}),
	)

	if err != nil {
		log.Printf("Warning: Error handling cookie dialog: %v", err)
		// Продолжаем выполнение, так как диалог может отсутствовать
	}

	// Ввод поискового запроса
	var searchInput string

	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("Entering search query: %s", query)
			return nil
		}),

		// Найти поле поиска (несколько вариантов селекторов)
		chromedp.Evaluate(`
			function findSearchInput() {
				const selectors = [
					"textarea[name='q']", 
					"input[title='Buscar']", 
					"input[title='Search']", 
					"input[name='q']", 
					"input[title='Axtar']"
				];
				
				for (const selector of selectors) {
					const input = document.querySelector(selector);
					if (input) return selector;
				}
				return null;
			}
			return findSearchInput();
		`, &searchInput),
	)

	if err != nil || searchInput == "" {
		return nil, fmt.Errorf("error finding search input: %w", err)
	}

	// Вводим поисковый запрос с человекоподобной скоростью
	err = chromedp.Run(ctx,
		chromedp.Focus(searchInput),

		// Имитируем ввод текста по одному символу
		chromedp.ActionFunc(func(ctx context.Context) error {
			for _, char := range query {
				if err := chromedp.SendKeys(searchInput, string(char)).Do(ctx); err != nil {
					return err
				}
				randomDelay(50, 150) // Случайная задержка между нажатиями клавиш
			}
			return nil
		}),

		chromedp.ActionFunc(func(ctx context.Context) error {
			randomDelay(500, 1200) // Пауза перед отправкой
			return nil
		}),

		// Отправляем форму
		chromedp.SendKeys(searchInput, string('\r')), // Enter key

		// Ждем загрузки результатов
		chromedp.WaitReady("#search"),
	)

	if err != nil {
		return nil, fmt.Errorf("error submitting search query: %w", err)
	}

	// Проверяем наличие анти-бот защиты
	var captchaDetected bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			!!document.querySelector('iframe[src*="recaptcha"]') || 
			!!document.querySelector('#captcha') ||
			!!document.querySelector('.g-recaptcha') ||
			document.title.includes('unusual traffic') ||
			document.body.innerText.includes('unusual traffic') ||
			document.body.innerText.includes('security check')
		`, &captchaDetected),
	)

	if err == nil && captchaDetected {
		log.Println("Anti-bot challenge detected! Please solve it manually.")
		// Ожидание 10 секунд для ручного решения
		time.Sleep(10 * time.Second)
	} else {
		// Небольшая пауза перед поиском раздела новостей
		randomDelay(2000, 4000)
	}

	// Находим и переходим в раздел новостей
	var foundNewsLink bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			function findNewsLink() {
				const selectors = [
					'a:has-text("Новости")', 
					'a:has-text("News")', 
					'a:has-text("Noticias")',
					'a:has-text("Xəbərlər")',
					'a:has-text("Actualités")',
					'a:has-text("Haberler")',
					'a[href*="tbm=nws"]',
					'a[data-hveid] div:contains("N")'
				];
				
				for (const selector of selectors) {
					try {
						const link = document.querySelector(selector);
						if (link) {
							console.log("Found news section:", link.textContent);
							link.click();
							return true;
						}
					} catch (e) {}
				}
				return false;
			}
			return findNewsLink();
		`, &foundNewsLink),
	)

	// Если не нашли раздел новостей, переходим напрямую по URL
	if err != nil || !foundNewsLink {
		log.Println("Could not find news section link, trying direct URL approach...")
		newsUrl := fmt.Sprintf("https://www.google.com/search?q=%s&tbm=nws", url.QueryEscape(query))

		err = chromedp.Run(ctx,
			chromedp.Navigate(newsUrl),
			chromedp.WaitReady("body"),
		)

		if err != nil {
			return nil, fmt.Errorf("error navigating directly to news: %w", err)
		}
	}

	// Ждем загрузки результатов новостей
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second),
	)

	if err != nil {
		return nil, fmt.Errorf("error waiting for news results: %w", err)
	}

	// Проходим по страницам новостей
	for pageNum := 1; pageNum <= maxPagesToCheck; pageNum++ {
		log.Printf("Processing page %d of %d", pageNum, maxPagesToCheck)

		// Разные селекторы для поиска заголовков новостей
		headingSelectors := []string{
			`div[role="heading"]`,
			`.n0jPhd`,
			`.MBeuO`,
			`article h3`,
			`h3`,
			`.DY5T1d`,
		}

		var foundHeadings bool
		var headingsSelector string

		// Проверяем каждый селектор
		for _, selector := range headingSelectors {
			var count int
			err = chromedp.Run(ctx,
				chromedp.Evaluate(fmt.Sprintf(`document.querySelectorAll('%s').length`, selector), &count),
			)

			if err == nil && count > 0 {
				log.Printf("Found %d elements with selector: %s", count, selector)
				foundHeadings = true
				headingsSelector = selector
				break
			}
		}

		if !foundHeadings {
			log.Printf("No headlines found on page %d", pageNum)

			if pageNum == 1 {
				// Если это первая страница и нет результатов, делаем скриншот и выходим
				var buf []byte
				err = chromedp.Run(ctx,
					chromedp.CaptureScreenshot(&buf),
					chromedp.ActionFunc(func(context.Context) error {
						if err := os.WriteFile("no_headlines.png", buf, 0644); err != nil {
							return err
						}
						log.Printf("No headlines found. Screenshot saved to no_headlines.png")
						return nil
					}),
				)
				return nil, fmt.Errorf("could not find any news headlines")
			} else {
				// Если это не первая страница, просто заканчиваем поиск
				log.Println("No more headlines found on additional pages")
				break
			}
		}

		// Получаем все заголовки на текущей странице
		err = chromedp.Run(ctx,
			chromedp.Nodes(headingsSelector, &titleNodes),
		)

		if err != nil {
			log.Printf("Error finding headlines on page %d: %v", pageNum, err)
			continue
		}

		log.Printf("Found %d news headlines on page %d, extracting details...", len(titleNodes), pageNum)

		// Обрабатываем новости по одной
		for i, node := range titleNodes {
			var newsItem NewsItem
			newsItem.Timestamp = time.Now()
			newsItem.Query = query

			// Получаем текст заголовка
			var title string
			err = chromedp.Run(ctx,
				chromedp.TextContent(cdp.NodeID(node.NodeID), &title),
			)

			if err != nil {
				log.Printf("Error getting title for item %d: %v", i+1, err)
				continue
			}

			newsItem.Title = strings.TrimSpace(title)
			log.Printf("Got title: %s", newsItem.Title)

			// Проверяем, является ли элемент новостью
			// Используем прямую подстановку переменных в JavaScript
			jsIsNewsItem := fmt.Sprintf(`
				function isNewsItem(selector, index) {
					const nodes = document.querySelectorAll("%s");
					if (!nodes || !nodes[%d]) return false;

					const el = nodes[%d];
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
				return isNewsItem();
			`, headingsSelector, i, i)

			var isRealNews bool
			err = chromedp.Run(ctx,
				chromedp.Evaluate(jsIsNewsItem, &isRealNews),
			)

			if err != nil {
				log.Printf("Error checking if item is news: %v", err)
				continue
			}

			// Пропускаем элементы, которые не являются новостями
			if !isRealNews {
				log.Printf("Skipping non-news item: %s", newsItem.Title)
				continue
			}

			// Получаем URL новости
			jsGetURL := fmt.Sprintf(`
				function getNewsUrl() {
					const nodes = document.querySelectorAll("%s");
					if (!nodes || !nodes[%d]) return "";

					const el = nodes[%d];
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
				return getNewsUrl();
			`, headingsSelector, i, i)

			err = chromedp.Run(ctx,
				chromedp.Evaluate(jsGetURL, &newsItem.URL),
			)

			if err != nil {
				log.Printf("Error getting URL: %v", err)
			} else {
				log.Printf("Got URL: %s", newsItem.URL)
			}

			// Получаем описание
			jsGetDescription := fmt.Sprintf(`
				function getDescription() {
					const nodes = document.querySelectorAll("%s");
					if (!nodes || !nodes[%d]) return "";

					const el = nodes[%d];
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
				return getDescription();
			`, headingsSelector, i, i)

			err = chromedp.Run(ctx,
				chromedp.Evaluate(jsGetDescription, &newsItem.Description),
			)

			if err != nil {
				log.Printf("Error getting description: %v", err)
			} else {
				newsItem.Description = strings.TrimSpace(newsItem.Description)
				log.Printf("Got description: %s", newsItem.Description)
			}

			// Получаем источник
			jsGetSource := fmt.Sprintf(`
				function getSource() {
					const nodes = document.querySelectorAll("%s");
					if (!nodes || !nodes[%d]) return "";

					const el = nodes[%d];
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
				return getSource();
			`, headingsSelector, i, i)

			err = chromedp.Run(ctx,
				chromedp.Evaluate(jsGetSource, &newsItem.Source),
			)

			if err != nil {
				log.Printf("Error getting source: %v", err)
			} else {
				newsItem.Source = strings.TrimSpace(newsItem.Source)
				log.Printf("Got source: %s", newsItem.Source)
			}

			// Добавляем новость в список результатов
			newsItems = append(newsItems, newsItem)

			// Проверяем, достигли ли мы целевого количества новостей
			if len(newsItems) >= targetNewsCount {
				log.Printf("Reached target count of %d news items", targetNewsCount)
				goto ScrapingCompleted
			}
		}

		// Если не последняя страница и нужно больше новостей
		if pageNum < maxPagesToCheck && len(newsItems) < targetNewsCount {
			// Ищем кнопку следующей страницы
			var hasNextPage bool

			err = chromedp.Run(ctx,
				chromedp.Evaluate(`
					function hasNext() {
						const nextButtons = document.querySelectorAll('[aria-label="Next page"], [aria-label="Siguiente página"], a:has-text("Next"), a:has-text("Siguiente")');
						return nextButtons.length > 0;
					}
					return hasNext();
				`, &hasNextPage),
			)

			if err != nil || !hasNextPage {
				log.Println("No more pages available")
				break
			}

			// Кликаем на кнопку следующей страницы
			log.Println("Found next page button, clicking...")

			err = chromedp.Run(ctx,
				chromedp.ActionFunc(func(ctx context.Context) error {
					randomDelay(500, 1000)
					return nil
				}),

				chromedp.Evaluate(`
					function clickNext() {
						const selectors = [
							'[aria-label="Next page"]', 
							'[aria-label="Siguiente página"]', 
							'a:has-text("Next")', 
							'a:has-text("Siguiente")'
						];
						
						for (const selector of selectors) {
							const btn = document.querySelector(selector);
							if (btn) {
								btn.click();
								return true;
							}
						}
						return false;
					}
					return clickNext();
				`, nil),

				// Ждем загрузки новой страницы
				chromedp.Sleep(3*time.Second),
				chromedp.WaitReady("body"),
			)

			if err != nil {
				log.Printf("Error clicking next page button: %v", err)
				break
			}

			// Очищаем nodes для новой страницы
			titleNodes = nil
		} else {
			// Достигли последней страницы или целевого количества новостей
			break
		}
	}

ScrapingCompleted:
	log.Printf("Scraping completed. Found %d news items.", len(newsItems))

	// Делаем финальный скриншот
	var buf []byte
	err = chromedp.Run(ctx,
		emulation.SetDeviceMetricsOverride(0, 0, 1.0, false),
		chromedp.CaptureScreenshot(&buf),
	)

	if err == nil {
		if err := os.WriteFile("final_results.png", buf, 0644); err == nil {
			log.Printf("Saved final screenshot to final_results.png")
		}
	}

	return newsItems, nil
}

// Пример использования в main.go
func main() {
	log.Println("Application starting...")

	// Настройка прокси из переменных окружения
	proxyServer := os.Getenv("PROXY_SERVER")
	if proxyServer == "" {
		// Пример настройки SOCKS5 прокси
		os.Setenv("PROXY_SERVER", "socks5://emh9gqay:4ap6ysrk@95.79.35.60:17320")
		os.Setenv("PROXY_USER", "emh9gqay") // если требуется аутентификация
		os.Setenv("PROXY_PASS", "4ap6ysrk") // если требуется аутентификация

		proxyServer = os.Getenv("PROXY_SERVER")
	}

	if proxyServer != "" {
		log.Printf("Using proxy: %s", proxyServer)
		if user := os.Getenv("PROXY_USER"); user != "" {
			log.Printf("Proxy with auth configured: %s", user)
		}
	}

	// Режим отображения браузера (headless или нет)
	headless := os.Getenv("HEADLESS") != "false"
	log.Printf("Running in browser %s mode (headless=%t)",
		map[bool]string{true: "hidden", false: "visible"}[headless],
		headless)

	// Поисковый запрос
	query := "RTErdogan"
	if q := os.Getenv("QUERY"); q != "" {
		query = q
	}
	log.Printf("Search query: %s", query)

	// Скрейпинг новостей
	log.Println("Starting Google News scraping...")
	results, err := ScrapeGoogleNews(query)

	if err != nil {
		log.Fatalf("Error scraping news: %v", err)
	}

	// Выводим результаты
	log.Printf("Found %d news items", len(results))
	for i, item := range results {
		log.Printf("%d. %s - %s", i+1, item.Title, item.Source)
	}
}
