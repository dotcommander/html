package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type metrics struct {
	URL                   string           `json:"url"`
	Title                 string           `json:"title"`
	ViewportW             int64            `json:"viewport_w"`
	ViewportH             int64            `json:"viewport_h"`
	Theme                 string           `json:"theme,omitempty"`
	Palette               string           `json:"palette,omitempty"`
	Accent                string           `json:"accent,omitempty"`
	SlideStatus           string           `json:"slide_status,omitempty"`
	CurrentSlide          string           `json:"current_slide,omitempty"`
	FilterStatus          string           `json:"filter_status,omitempty"`
	FirstRow              string           `json:"first_row,omitempty"`
	SortState             string           `json:"sort_state,omitempty"`
	SortLabel             string           `json:"sort_label,omitempty"`
	MobileSort            string           `json:"mobile_sort,omitempty"`
	EmptyRowText          string           `json:"empty_row_text,omitempty"`
	EmptyRowVisible       bool             `json:"empty_row_visible,omitempty"`
	SelectedTab           string           `json:"selected_tab,omitempty"`
	VisibleTabPanel       string           `json:"visible_tab_panel,omitempty"`
	ThemeControlsDisplay  string           `json:"theme_controls_display,omitempty"`
	ThemeControlsHeight   float64          `json:"theme_controls_height,omitempty"`
	ThemeControlsOneRow   bool             `json:"theme_controls_one_row,omitempty"`
	TOCCount              int64            `json:"toc_count,omitempty"`
	MarkdownMaxWidth      string           `json:"markdown_max_width,omitempty"`
	ConsoleErrors         int              `json:"console_errors"`
	ConsoleWarnings       int              `json:"console_warnings"`
	ConsoleMessages       []string         `json:"console_messages,omitempty"`
	IframeCount           int64            `json:"iframe_count,omitempty"`
	IframeLoadedFrames    int64            `json:"iframe_loaded_frames,omitempty"`
	IframeTextFrames      int64            `json:"iframe_text_frames,omitempty"`
	IframeThemePalettes   []string         `json:"iframe_theme_palettes"`
	DashboardCards        int64            `json:"dashboard_cards,omitempty"`
	DashboardCardCounts   map[string]int64 `json:"dashboard_card_counts,omitempty"`
	DashboardSuiteHTML    int64            `json:"dashboard_suite_html,omitempty"`
	DashboardDetection    string           `json:"dashboard_detection_proof,omitempty"`
	CoverageMissing       int64            `json:"coverage_missing"`
	PlanContracts         int64            `json:"plan_contracts,omitempty"`
	PlanContractLayout    string           `json:"plan_contract_layout,omitempty"`
	PlanContractComponent string           `json:"plan_contract_component,omitempty"`
	CacheContracts        int64            `json:"cache_contracts,omitempty"`
	CacheReused           string           `json:"cache_reused,omitempty"`
	CacheForced           string           `json:"cache_forced,omitempty"`
	ComponentCounts       map[string]int64 `json:"component_counts,omitempty"`
	ReportDataRows        int64            `json:"report_data_rows,omitempty"`
	VisibleReportRows     int64            `json:"visible_report_rows,omitempty"`
	RecordCardItems       int64            `json:"record_card_items,omitempty"`
	ReportTabButtons      int64            `json:"report_tab_buttons,omitempty"`
	ReportSlideItems      int64            `json:"report_slide_items,omitempty"`
	JSONOverviewItems     int64            `json:"json_overview_items,omitempty"`
	DiffRenderedLines     int64            `json:"diff_rendered_lines,omitempty"`
	DiffAddedLines        int64            `json:"diff_added_lines,omitempty"`
	DiffRemovedLines      int64            `json:"diff_removed_lines,omitempty"`
	FileTreeItems         int64            `json:"file_tree_items,omitempty"`
	LogLineItems          int64            `json:"log_line_items,omitempty"`
	LogSeverityItems      int64            `json:"log_severity_items,omitempty"`
	CodeOverviewItems     int64            `json:"code_overview_items,omitempty"`
	ChromaLineItems       int64            `json:"chroma_line_items,omitempty"`
	ReportTextLines       int64            `json:"report_text_lines,omitempty"`
	BinaryPreviewLines    int64            `json:"binary_preview_lines,omitempty"`
	DetectionRows         int64            `json:"detection_rows,omitempty"`
	DetectionKindCounts   map[string]int64 `json:"detection_kind_counts,omitempty"`
	ErrorContracts        int64            `json:"error_contracts,omitempty"`
	CopyButtons           int64            `json:"copy_buttons,omitempty"`
	HeadingAnchors        int64            `json:"heading_anchors,omitempty"`
	ReportTextBlocks      int64            `json:"report_text_blocks,omitempty"`
	TextOverviews         int64            `json:"text_overviews,omitempty"`
	TranscriptTurns       int64            `json:"transcript_turns,omitempty"`
	TranscriptSpeakers    int64            `json:"transcript_speakers,omitempty"`
	PlaintextBlocks       int64            `json:"plaintext_blocks,omitempty"`
	ANSICodeBlocks        int64            `json:"ansi_code_blocks,omitempty"`
	ANSIStyledSpans       int64            `json:"ansi_styled_spans,omitempty"`
	ANSILines             int64            `json:"ansi_lines,omitempty"`
	TermFrameBars         int64            `json:"term_frame_bars,omitempty"`
	TermFrameBodyBlocks   int64            `json:"term_frame_body_blocks,omitempty"`
	TaskCheckboxes        int64            `json:"task_checkboxes,omitempty"`
	Blockquotes           int64            `json:"blockquotes,omitempty"`
	AlertTextPresent      bool             `json:"alert_text_present"`
	ImageCount            int64            `json:"image_count,omitempty"`
	LoadedImages          int64            `json:"loaded_images,omitempty"`
	DataURIImages         int64            `json:"data_uri_images,omitempty"`
	SVGImages             int64            `json:"svg_images,omitempty"`
	RasterImages          int64            `json:"raster_images,omitempty"`
	MediaPreviewImages    int64            `json:"media_preview_images,omitempty"`
	MediaPreviewSource    int64            `json:"media_preview_source_images,omitempty"`
	MediaPreviewRendered  int64            `json:"media_preview_rendered_images,omitempty"`
	ClientWidth           int64            `json:"client_width"`
	ScrollWidth           int64            `json:"scroll_width"`
	BodyWidth             int64            `json:"body_width"`
	DocHeight             int64            `json:"doc_height"`
}

func main() {
	url := flag.String("url", "", "URL to capture")
	out := flag.String("out", "", "PNG output path")
	width := flag.Int64("width", 390, "viewport width")
	height := flag.Int64("height", 900, "viewport height")
	palette := flag.String("palette", "", "palette button to click before capture")
	clickThemeToggle := flag.Bool("click-theme-toggle", false, "click the light/dark theme toggle before capture")
	clickSlideNext := flag.Bool("click-slide-next", false, "click the first report slide next button before capture")
	filter := flag.String("filter", "", "table filter text to type before capture")
	sortHeader := flag.String("sort-header", "", "table header text to click before capture")
	mobileSort := flag.String("mobile-sort", "", "mobile sort select value, for example 1:descending")
	clickTab := flag.String("click-tab", "", "tab button text to click before capture")
	flag.Parse()

	if *url == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: chromedp-capture --url URL --out file.png [--width W --height H]")
		os.Exit(2)
	}

	chrome := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	profile, err := os.MkdirTemp("", "html-chromedp-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp profile: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(profile)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chrome),
		chromedp.UserDataDir(profile),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-crash-reporter", true),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var png []byte
	var title string
	var m metrics
	var consoleMu sync.Mutex
	m.URL = *url
	m.ViewportW = *width
	m.ViewportH = *height

	chromedp.ListenTarget(ctx, func(ev any) {
		consoleMu.Lock()
		defer consoleMu.Unlock()
		switch e := ev.(type) {
		case *cdpruntime.EventConsoleAPICalled:
			switch e.Type {
			case cdpruntime.APITypeError, cdpruntime.APITypeAssert:
				m.ConsoleErrors++
				m.ConsoleMessages = appendConsoleMessage(m.ConsoleMessages, "console."+e.Type.String()+": "+remoteArgs(e.Args))
			case cdpruntime.APITypeWarning:
				m.ConsoleWarnings++
				m.ConsoleMessages = appendConsoleMessage(m.ConsoleMessages, "console.warning: "+remoteArgs(e.Args))
			}
		case *cdpruntime.EventExceptionThrown:
			m.ConsoleErrors++
			msg := "exception"
			if e.ExceptionDetails != nil {
				msg = strings.TrimSpace(e.ExceptionDetails.Error())
			}
			m.ConsoleMessages = appendConsoleMessage(m.ConsoleMessages, msg)
		}
	})

	if err := chromedp.Run(ctx,
		cdpruntime.Enable(),
		emulation.SetDeviceMetricsOverride(*width, *height, 1, false),
		emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{
			{Name: "prefers-color-scheme", Value: "light"},
		}),
		chromedp.Navigate(*url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(750*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !*clickThemeToggle {
				return nil
			}
			return chromedp.Click(`#theme-toggle`, chromedp.ByID).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if *palette == "" {
				return nil
			}
			return chromedp.Click(`[data-palette-choice="`+*palette+`"]`, chromedp.ByQuery).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !*clickSlideNext {
				return nil
			}
			return chromedp.Click(`[data-slide-next]`, chromedp.ByQuery).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if *clickTab == "" {
				return nil
			}
			sel := fmt.Sprintf(`//button[@role="tab" and normalize-space(.)=%q]`, *clickTab)
			return chromedp.Click(sel, chromedp.BySearch).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if *filter == "" {
				return nil
			}
			return chromedp.SendKeys(`.report-filter`, *filter, chromedp.ByQuery).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if *sortHeader == "" {
				return nil
			}
			sel := fmt.Sprintf(`//th/button[normalize-space(.)=%q]`, *sortHeader)
			return chromedp.Click(sel, chromedp.BySearch).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if *mobileSort == "" {
				return nil
			}
			expr := fmt.Sprintf(`(() => {
				const select = document.querySelector(".report-mobile-sort select");
				if (!select) return false;
				select.value = %q;
				select.dispatchEvent(new Event("change", { bubbles: true }));
				return true;
			})()`, *mobileSort)
			var ok bool
			if err := chromedp.Evaluate(expr, &ok).Do(ctx); err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("mobile sort select not found")
			}
			return nil
		}),
		chromedp.Title(&title),
		chromedp.Evaluate(`document.documentElement.dataset.theme || ""`, &m.Theme),
		chromedp.Evaluate(`document.documentElement.dataset.palette || ""`, &m.Palette),
		chromedp.Evaluate(`getComputedStyle(document.documentElement).getPropertyValue("--accent").trim()`, &m.Accent),
		chromedp.Evaluate(`document.querySelector("[data-slide-status]")?.textContent || ""`, &m.SlideStatus),
		chromedp.Evaluate(`document.querySelector(".report-slide[aria-current='true']")?.getAttribute("aria-label") || ""`, &m.CurrentSlide),
		chromedp.Evaluate(`document.querySelector(".report-filter-status")?.textContent || ""`, &m.FilterStatus),
		chromedp.Evaluate(`Array.from(document.querySelectorAll("[data-report-table] tbody tr")).find((row) => !row.hidden)?.textContent.trim().replace(/\s+/g, " ") || ""`, &m.FirstRow),
		chromedp.Evaluate(`document.querySelector("[aria-sort]")?.getAttribute("aria-sort") || ""`, &m.SortState),
		chromedp.Evaluate(`document.querySelector("[aria-sort] button")?.getAttribute("aria-label") || ""`, &m.SortLabel),
		chromedp.Evaluate(`document.querySelector(".report-mobile-sort select")?.value || ""`, &m.MobileSort),
		chromedp.Evaluate(`document.querySelector("[data-report-empty-row]")?.textContent.trim().replace(/\s+/g, " ") || ""`, &m.EmptyRowText),
		chromedp.Evaluate(`(() => {
			const row = document.querySelector("[data-report-empty-row]");
			return Boolean(row && !row.hidden && getComputedStyle(row).display !== "none");
		})()`, &m.EmptyRowVisible),
		chromedp.Evaluate(`document.querySelector('[role="tab"][aria-selected="true"]')?.textContent.trim().replace(/\s+/g, " ") || ""`, &m.SelectedTab),
		chromedp.Evaluate(`document.querySelector('[role="tabpanel"]:not([hidden])')?.textContent.trim().replace(/\s+/g, " ").slice(0, 80) || ""`, &m.VisibleTabPanel),
		chromedp.Evaluate(`(() => {
			const controls = document.querySelector(".theme-controls");
			return controls ? getComputedStyle(controls).display : "";
		})()`, &m.ThemeControlsDisplay),
		chromedp.Evaluate(`document.querySelector(".theme-controls")?.getBoundingClientRect().height || 0`, &m.ThemeControlsHeight),
		chromedp.Evaluate(`(() => {
			const controls = document.querySelector(".theme-controls");
			if (!controls) return false;
			const items = Array.from(controls.querySelectorAll("button"));
			if (items.length < 2) return false;
			const top = items[0].getBoundingClientRect().top;
			return items.every((item) => Math.abs(item.getBoundingClientRect().top - top) < 2);
		})()`, &m.ThemeControlsOneRow),
		chromedp.Evaluate(`document.querySelectorAll("nav.toc").length`, &m.TOCCount),
		chromedp.Evaluate(`getComputedStyle(document.querySelector(".markdown-body") || document.body).maxWidth || ""`, &m.MarkdownMaxWidth),
		chromedp.Evaluate(`document.querySelectorAll("iframe").length`, &m.IframeCount),
		chromedp.Evaluate(`document.querySelectorAll("iframe[data-loaded='true']").length`, &m.IframeLoadedFrames),
		chromedp.Evaluate(`Array.from(document.querySelectorAll("iframe")).filter((frame) => {
			try {
				return Boolean(frame.contentDocument && frame.contentDocument.body && frame.contentDocument.body.innerText.trim());
			} catch (_) {
				return false;
			}
		}).length`, &m.IframeTextFrames),
		chromedp.Evaluate(`(() => {
			const declared = Array.from(document.querySelectorAll("[data-theme-case]"))
				.map((node) => node.getAttribute("data-theme-case") || "")
				.filter(Boolean);
			if (declared.length) return declared;
			return Array.from(document.querySelectorAll("iframe")).map((frame) => {
				try {
					const root = frame.contentDocument && frame.contentDocument.documentElement;
					if (!root) return "";
					const html = root.innerHTML || "";
					const theme = root.dataset.theme || html.match(/HTML_DEFAULT_THEME\s*=\s*"([^"]+)"/)?.[1] || "";
					const palette = root.dataset.palette || html.match(/HTML_DEFAULT_PALETTE\s*=\s*"([^"]+)"/)?.[1] || "";
					return theme && palette ? theme + "/" + palette : "";
				} catch (_) {
					return "";
				}
			}).filter(Boolean);
		})()`, &m.IframeThemePalettes),
		chromedp.Evaluate(`document.querySelectorAll("[data-dashboard-card]").length`, &m.DashboardCards),
		chromedp.Evaluate(`(() => {
			const counts = {};
			document.querySelectorAll("[data-dashboard-card]").forEach((card) => {
				const title = card.dataset.dashboardTitle || card.querySelector("h2")?.textContent.trim() || "";
				const text = card.querySelector("header p")?.textContent || "";
				const match = text.match(/\d+/);
				if (title && match) counts[title] = Number(match[0]);
			});
			return counts;
		})()`, &m.DashboardCardCounts),
		chromedp.Evaluate(`Number(document.querySelector('[data-dashboard-total="suite-html"] dd')?.textContent.trim() || 0)`, &m.DashboardSuiteHTML),
		chromedp.Evaluate(`document.querySelector('[data-dashboard-title="Detection Matrix"] dd')?.textContent.trim().replace(/\s+/g, " ") || ""`, &m.DashboardDetection),
		chromedp.Evaluate(`document.querySelectorAll(".coverage-list .missing").length`, &m.CoverageMissing),
		chromedp.Evaluate(`document.querySelectorAll("[data-plan-contract]").length`, &m.PlanContracts),
		chromedp.Evaluate(`document.querySelector("[data-plan-contract]")?.getAttribute("data-plan-layout") || ""`, &m.PlanContractLayout),
		chromedp.Evaluate(`document.querySelector("[data-plan-contract]")?.getAttribute("data-plan-component") || ""`, &m.PlanContractComponent),
		chromedp.Evaluate(`document.querySelectorAll("[data-cache-contract]").length`, &m.CacheContracts),
		chromedp.Evaluate(`document.querySelector("[data-cache-contract]")?.getAttribute("data-cache-reused") || ""`, &m.CacheReused),
		chromedp.Evaluate(`document.querySelector("[data-cache-contract]")?.getAttribute("data-cache-forced") || ""`, &m.CacheForced),
		chromedp.Evaluate(`(() => {
			const selectors = {
				markdown_body: ".markdown-body",
				markdown_table: ".markdown-body table:not(.report-table)",
				markdown_alert: ".markdown-alert",
				plain_data_table: ".plain-data-table",
				plain_table_section: ".plain-table-section",
				plain_table_meta: ".plain-table-meta",
				article_overview: ".article-overview",
				report_summary: ".report-summary",
				report_table: ".report-table",
				record_cards: ".record-cards",
				report_tabs: ".report-tabs",
				report_slides: ".report-slides",
				diff_view: ".diff-view",
				file_tree: ".file-tree",
				log_lines: ".log-lines",
				binary_preview: ".binary-preview",
					code_overview: ".code-overview",
					json_overview: ".json-overview",
					json_source: ".json-source",
					chroma: ".chroma",
					term_frame: ".term-frame"
			};
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return Object.fromEntries(Object.entries(selectors).map(([key, selector]) => [
				key,
				docs.reduce((sum, doc) => sum + doc.querySelectorAll(selector).length, 0)
			]));
		})()`, &m.ComponentCounts),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("[data-report-table] tbody tr")).filter((row) => !row.hasAttribute("data-report-empty-row")).length, 0);
		})()`, &m.ReportDataRows),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("[data-report-table] tbody tr")).filter((row) => !row.hasAttribute("data-report-empty-row") && !row.hidden && getComputedStyle(row).display !== "none").length, 0);
		})()`, &m.VisibleReportRows),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".record-card").length, 0);
		})()`, &m.RecordCardItems),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll('[role="tab"]').length, 0);
		})()`, &m.ReportTabButtons),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".report-slide").length, 0);
			})()`, &m.ReportSlideItems),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll(".json-overview")).reduce((inner, overview) => inner + overview.querySelectorAll("div, span").length, 0), 0);
			})()`, &m.JSONOverviewItems),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".diff-view code > span").length, 0);
			})()`, &m.DiffRenderedLines),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".diff-view code > span.add").length, 0);
			})()`, &m.DiffAddedLines),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".diff-view code > span.del").length, 0);
			})()`, &m.DiffRemovedLines),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".file-tree li").length, 0);
			})()`, &m.FileTreeItems),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".log-lines .log-line").length, 0);
			})()`, &m.LogLineItems),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".log-lines .log-level").length, 0);
			})()`, &m.LogSeverityItems),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll(".code-overview")).reduce((inner, overview) => inner + overview.querySelectorAll("div").length, 0), 0);
			})()`, &m.CodeOverviewItems),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll("pre.chroma .line, pre.chroma .cl").length, 0);
			})()`, &m.ChromaLineItems),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("pre.report-text code")).reduce((inner, code) => {
					const text = code.textContent || "";
					return inner + (text === "" ? 0 : text.split(/\n/).length);
				}, 0), 0);
			})()`, &m.ReportTextLines),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("pre.binary-preview code")).reduce((inner, code) => {
					const text = code.textContent || "";
					return inner + (text === "" ? 0 : text.split(/\n/).length);
				}, 0), 0);
			})()`, &m.BinaryPreviewLines),
		chromedp.Evaluate(`document.querySelectorAll("tr[data-kind]").length`, &m.DetectionRows),
		chromedp.Evaluate(`(() => {
			const counts = {};
			document.querySelectorAll("tr[data-kind]").forEach((row) => {
				const kind = row.dataset.kind || "";
				if (kind) counts[kind] = (counts[kind] || 0) + 1;
			});
			return counts;
		})()`, &m.DetectionKindCounts),
		chromedp.Evaluate(`document.querySelectorAll("[data-error-contract]").length`, &m.ErrorContracts),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".copy-btn").length, 0);
		})()`, &m.CopyButtons),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".heading-anchor").length, 0);
		})()`, &m.HeadingAnchors),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll("pre.report-text").length, 0);
		})()`, &m.ReportTextBlocks),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".text-overview").length, 0);
		})()`, &m.TextOverviews),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".transcript-turn").length, 0);
			})()`, &m.TranscriptTurns),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".transcript-speaker").length, 0);
			})()`, &m.TranscriptSpeakers),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll("pre code.language-plaintext").length, 0);
		})()`, &m.PlaintextBlocks),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll("pre code.language-ansi").length, 0);
			})()`, &m.ANSICodeBlocks),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll("pre code.language-ansi span[style]").length, 0);
			})()`, &m.ANSIStyledSpans),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("pre code.language-ansi")).reduce((inner, code) => {
					const text = code.textContent || "";
					return inner + (text === "" ? 0 : text.split(/\n/).length);
				}, 0), 0);
			})()`, &m.ANSILines),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".term-frame .term-bar").length, 0);
			})()`, &m.TermFrameBars),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
					if (!doc || docs.includes(doc)) return;
					docs.push(doc);
					for (const frame of doc.querySelectorAll("iframe")) {
						try {
							if (frame.contentDocument) visit(frame.contentDocument);
						} catch (_) {}
					}
				};
				visit(document);
				return docs.reduce((sum, doc) => sum + doc.querySelectorAll(".term-frame .term-body pre").length, 0);
			})()`, &m.TermFrameBodyBlocks),
		chromedp.Evaluate(`(() => {
				const docs = [];
				const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll("input[type='checkbox']").length, 0);
		})()`, &m.TaskCheckboxes),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll("blockquote").length, 0);
		})()`, &m.Blockquotes),
		chromedp.Evaluate(`(() => {
			const docs = [];
			const visit = (doc) => {
				if (!doc || docs.includes(doc)) return;
				docs.push(doc);
				for (const frame of doc.querySelectorAll("iframe")) {
					try {
						if (frame.contentDocument) visit(frame.contentDocument);
					} catch (_) {}
				}
			};
			visit(document);
			return docs.some((doc) => (doc.body?.innerText || "").includes("alert(1)"));
		})()`, &m.AlertTextPresent),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll("img").length, 0);
		})()`, &m.ImageCount),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("img")).filter((img) => img.complete && img.naturalWidth > 0 && img.naturalHeight > 0).length, 0);
		})()`, &m.LoadedImages),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("img")).filter((img) => img.currentSrc.startsWith("data:image/") || img.getAttribute("src")?.startsWith("data:image/")).length, 0);
		})()`, &m.DataURIImages),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("img")).filter((img) => {
				const src = img.currentSrc || img.getAttribute("src") || "";
				return src.endsWith(".svg") || src.startsWith("data:image/svg+xml");
			}).length, 0);
		})()`, &m.SVGImages),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + Array.from(doc.querySelectorAll("img")).filter((img) => {
				const src = img.currentSrc || img.getAttribute("src") || "";
				return src.endsWith(".png") || src.startsWith("data:image/png");
			}).length, 0);
		})()`, &m.RasterImages),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll("[data-media-preview] img").length, 0);
		})()`, &m.MediaPreviewImages),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll('[data-media-preview="source"] img').length, 0);
		})()`, &m.MediaPreviewSource),
		chromedp.Evaluate(`(() => {
			const docs = [document];
			for (const frame of document.querySelectorAll("iframe")) {
				try {
					if (frame.contentDocument) docs.push(frame.contentDocument);
				} catch (_) {}
			}
			return docs.reduce((sum, doc) => sum + doc.querySelectorAll('[data-media-preview="rendered"] img').length, 0);
		})()`, &m.MediaPreviewRendered),
		chromedp.Evaluate(`document.documentElement.clientWidth`, &m.ClientWidth),
		chromedp.Evaluate(`document.documentElement.scrollWidth`, &m.ScrollWidth),
		chromedp.Evaluate(`document.body.scrollWidth`, &m.BodyWidth),
		chromedp.Evaluate(`Math.max(document.body.scrollHeight, document.documentElement.scrollHeight)`, &m.DocHeight),
		chromedp.FullScreenshot(&png, 100),
	); err != nil {
		fmt.Fprintf(os.Stderr, "capture: %v\n", err)
		os.Exit(1)
	}
	m.Title = title

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, png, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write png: %v\n", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(m); err != nil {
		fmt.Fprintf(os.Stderr, "json: %v\n", err)
		os.Exit(1)
	}
}

func appendConsoleMessage(messages []string, message string) []string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "(empty console message)"
	}
	if len(message) > 220 {
		message = message[:220] + "..."
	}
	if len(messages) >= 8 {
		return messages
	}
	return append(messages, message)
}

func remoteArgs(args []*cdpruntime.RemoteObject) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == nil:
			continue
		case arg.Description != "":
			parts = append(parts, arg.Description)
		case len(arg.Value) > 0:
			parts = append(parts, strings.Trim(string(arg.Value), `"`))
		case arg.UnserializableValue != "":
			parts = append(parts, arg.UnserializableValue.String())
		}
	}
	return strings.Join(parts, " ")
}
