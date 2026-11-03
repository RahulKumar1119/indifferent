package storymetrics

import (
	"encoding/json"
	"fmt"
)

// DashboardName is the CloudWatch dashboard that shows the five metrics.
const DashboardName = "indifferent-fun-story"

// definitions is the text at the top of the dashboard. A rate nobody can read the
// definition of is easy to misread, so it sits next to the numbers.
const definitions = `## Story2Shorts success metrics (issue #194)

Rates are per day, so a quiet day is noisy. Counts are recorded when something happens and delivered at least once, so a rate is a guide, not an audit. Nothing here identifies a user or a story.

1. **Story to video**: finished videos ÷ stories submitted (plans made + plans that failed). A story that was planned and never turned into a video lowers it.
2. **Generation time**: from "Make the video" to the finished file, including waiting for a task to start.
3. **Stock coverage**: searches that found something ÷ searches. The product does not pick a picture for a scene: the picker searches the scene's keywords and the user chooses. So this counts searches, including ones the user typed, not scenes.
4. **Render failure rate**: failed ÷ (finished + failed). A video the user cancels is not a failure.
5. **Replacement rate**: pictures swapped or removed ÷ pictures chosen. Nothing is suggested, so this is how often a chosen picture changes later.`

type widget map[string]any

func ns(name string, opts map[string]any) []any {
	m := []any{Namespace, name}
	if opts != nil {
		m = append(m, opts)
	}
	return m
}

func expr(id, label, expression string) []any {
	return []any{map[string]any{"expression": expression, "label": label, "id": id}}
}

// hidden is a metric used only inside an expression.
func hidden(id, name, stat string) []any {
	return ns(name, map[string]any{"id": id, "stat": stat, "visible": false})
}

// rateWidget draws "100 × numerator ÷ denominator" as a daily percentage. Both
// formulas are written out by the caller so each rate can be read, and checked,
// on its own. FILL makes a missing count 0, so a day with a denominator and no
// events reads 0%; a day with no denominator divides by zero, which CloudWatch
// drops, so it shows as a gap and not as a false 0%.
func rateWidget(x, y int, title, label, formula string, series ...[]any) widget {
	metrics := [][]any{expr("rate", label, formula)}
	metrics = append(metrics, series...)
	return widget{
		"type": "metric", "x": x, "y": y, "width": 8, "height": 6,
		"properties": map[string]any{
			"title": title, "view": "timeSeries", "stacked": false, "region": "", "period": 86400, "stat": "Sum",
			"metrics": metrics, "yAxis": map[string]any{"left": map[string]any{"min": 0, "max": 100, "label": "%"}},
		},
	}
}

// Dashboard returns the body of the CloudWatch dashboard for region. Every
// metric it names is one that Names() lists; a test checks it.
func Dashboard(region string) ([]byte, error) {
	w := []widget{
		{"type": "text", "x": 0, "y": 0, "width": 24, "height": 8, "properties": map[string]any{"markdown": definitions}},

		// 1. Story to video: finished videos / stories submitted.
		rateWidget(0, 8, "1. Story to video (% of stories submitted)", "Story to video",
			"100 * FILL(done, 0) / (FILL(plans, 0) + FILL(failedplans, 0))",
			hidden("done", VideosCompleted, "Sum"), hidden("plans", PlansReady, "Sum"), hidden("failedplans", PlansFailed, "Sum")),
		// 2. Generation time.
		{"type": "metric", "x": 8, "y": 8, "width": 8, "height": 6, "properties": map[string]any{
			"title": "2. Generation time (seconds)", "view": "timeSeries", "stacked": false, "region": "", "period": 86400,
			"metrics": [][]any{
				ns(VideoSeconds, map[string]any{"stat": "Average", "label": "Average"}),
				ns(VideoSeconds, map[string]any{"stat": "p90", "label": "Slowest 10%"}),
				ns(VideoSeconds, map[string]any{"stat": "Maximum", "label": "Slowest"}),
			},
			"yAxis": map[string]any{"left": map[string]any{"min": 0, "label": "seconds"}},
		}},
		// 3. Stock coverage.
		{"type": "metric", "x": 16, "y": 8, "width": 8, "height": 6, "properties": map[string]any{
			"title": "3. Stock coverage (% of searches that found something)", "view": "timeSeries", "stacked": false, "region": "", "period": 86400, "stat": "Sum",
			"metrics": [][]any{
				expr("coverage", "Searches that found something", "100 * (FILL(searches, 0) - FILL(empty, 0)) / (searches)"),
				hidden("searches", StockSearches, "Sum"),
				hidden("empty", StockSearchesEmpty, "Sum"),
			},
			"yAxis": map[string]any{"left": map[string]any{"min": 0, "max": 100, "label": "%"}},
		}},
		// 4. Render failure rate: failed / (finished + failed).
		rateWidget(0, 14, "4. Render failure rate (% of videos that ended)", "Failed",
			"100 * FILL(failed, 0) / (FILL(failed, 0) + FILL(done, 0))",
			hidden("failed", VideosFailed, "Sum"), hidden("done", VideosCompleted, "Sum")),
		// 5. Replacement rate: (swapped + removed) / chosen.
		rateWidget(8, 14, "5. Replacement rate (% of chosen pictures later swapped or removed)", "Swapped or removed",
			"100 * (FILL(swapped, 0) + FILL(removed, 0)) / (chosen)",
			hidden("swapped", PicturesReplaced, "Sum"), hidden("removed", PicturesRemoved, "Sum"), hidden("chosen", PicturesChosen, "Sum")),

		// The counts behind the rates.
		{"type": "metric", "x": 16, "y": 14, "width": 8, "height": 6, "properties": map[string]any{
			"title": "Videos (counts)", "view": "timeSeries", "stacked": false, "region": "", "period": 86400, "stat": "Sum",
			"metrics": [][]any{
				ns(VideosStarted, map[string]any{"label": "Started"}),
				ns(VideosCompleted, map[string]any{"label": "Finished"}),
				ns(VideosFailed, map[string]any{"label": "Failed"}),
				ns(VideosCancelled, map[string]any{"label": "Cancelled"}),
			},
		}},
		{"type": "metric", "x": 0, "y": 20, "width": 12, "height": 6, "properties": map[string]any{
			"title": "Failures by reason", "view": "timeSeries", "stacked": true, "region": "", "period": 86400, "stat": "Sum",
			"metrics": [][]any{
				{map[string]any{"expression": fmt.Sprintf(`SEARCH('{%s,%s} MetricName="%s"', 'Sum', 86400)`, Namespace, CodeDimension, VideosFailed), "id": "bycode", "label": ""}},
			},
		}},
		{"type": "metric", "x": 12, "y": 20, "width": 12, "height": 6, "properties": map[string]any{
			"title": "Stories and edits (counts)", "view": "timeSeries", "stacked": false, "region": "", "period": 86400, "stat": "Sum",
			"metrics": [][]any{
				ns(PlansReady, map[string]any{"label": "Plans made"}),
				ns(PlansFailed, map[string]any{"label": "Plans that failed"}),
				ns(PicturesChosen, map[string]any{"label": "Pictures chosen"}),
				ns(PicturesReplaced, map[string]any{"label": "Pictures swapped"}),
				ns(PicturesRemoved, map[string]any{"label": "Pictures removed"}),
				ns(NarrationsEdited, map[string]any{"label": "Narrations edited"}),
				ns(StockSearches, map[string]any{"label": "Searches"}),
			},
		}},
	}
	// Every metric widget says which region its metrics are in.
	for _, x := range w {
		if props, ok := x["properties"].(map[string]any); ok {
			if _, has := props["region"]; has {
				props["region"] = region
			}
		}
	}
	return json.MarshalIndent(map[string]any{"start": "-P14D", "widgets": w}, "", "  ")
}
