package report

import (
	"regexp"
	"strings"
)

var (
	reTimestamp         = regexp.MustCompile(`(?m)(^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}|^\d{2}:\d{2}:\d{2})`)
	reSeverity          = regexp.MustCompile(`(?i)\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PASS|FAIL|panic:)\b`)
	reGoTest            = regexp.MustCompile(`(?m)^(ok|FAIL|\?)\s+\S+\s+((\d+(\.\d+)?s)|\(cached\)|\[no test files\])\s*$`)
	reGoTestMark        = regexp.MustCompile(`(?m)^(--- FAIL:|PASS$|FAIL$)`)
	reAccessLog         = regexp.MustCompile(`^\S+\s+\S+\s+\S+\s+\[[^\]]+\]\s+"[A-Z]+ [^"]*(?: HTTP/\d(?:\.\d)?)?"\s+[1-5]\d\d\s+(?:\d+|-)(?:\s|$)`)
	reTranscriptSpeaker = regexp.MustCompile(`^(?:Speaker [0-9A-Za-z]+|Host|Guest|Interviewer|Interviewee|Participant [0-9A-Za-z]+|[A-Z][A-Za-z .'-]{1,40}):\s+\S`)
)

func analyzeGoTestLog(text string, stats Stats) (Analysis, bool) {
	if reGoTest.MatchString(text) || reGoTestMark.MatchString(text) || strings.Contains(text, "\nFAIL\t") {
		return Analysis{Kind: KindLog, Confidence: 0.82, Reasons: []string{"log/test/console markers"}, Stats: stats}, true
	}
	return Analysis{}, false
}

func analyzeAccessLog(text string, stats Stats) (Analysis, bool) {
	lines := nonEmptyLines(text)
	if len(lines) == 0 {
		return Analysis{}, false
	}
	matches := 0
	for _, line := range lines {
		if reAccessLog.MatchString(strings.TrimSpace(line)) {
			matches++
		}
	}
	if matches == 0 {
		return Analysis{}, false
	}
	if matches == len(lines) || matches >= 2 && float64(matches)/float64(len(lines)) >= 0.6 {
		return Analysis{Kind: KindLog, Confidence: 0.84, Reasons: []string{"http access log markers"}, Stats: stats}, true
	}
	return Analysis{}, false
}

func analyzeTranscript(text string, stats Stats) (Analysis, bool) {
	lines := nonEmptyLines(text)
	if len(lines) < 3 {
		return Analysis{}, false
	}
	turns := 0
	speakers := map[string]bool{}
	explicitSpeakers := map[string]bool{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !reTranscriptSpeaker.MatchString(line) {
			continue
		}
		label, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		turns++
		speakers[label] = true
		if strings.HasPrefix(label, "Speaker ") || strings.HasPrefix(label, "Participant ") || label == "Host" || label == "Guest" || label == "Interviewer" || label == "Interviewee" {
			explicitSpeakers[label] = true
		}
	}
	repeatedSpeakerTurns := turns > len(speakers)
	if turns >= 3 && float64(turns)/float64(len(lines)) >= 0.6 && len(speakers) >= 2 && (len(explicitSpeakers) >= 2 || repeatedSpeakerTurns) {
		stats.Records = turns
		stats.Fields = len(speakers)
		return Analysis{Kind: KindTranscript, Confidence: 0.84, Reasons: []string{"speaker-labeled transcript turns"}, Stats: stats}, true
	}
	return Analysis{}, false
}

func analyzeLog(text string, stats Stats) (Analysis, bool) {
	lines := nonEmptyLines(text)
	if len(lines) == 0 {
		return Analysis{}, false
	}
	score := 0
	if reTimestamp.MatchString(text) {
		score++
	}
	if reSeverity.MatchString(text) {
		score++
	}
	if reGoTestMark.MatchString(text) || strings.Contains(text, "\nFAIL\t") {
		score++
	}
	if reGoTest.MatchString(text) {
		score += 2
	}
	if strings.Contains(text, "$ ") || strings.Contains(text, "> ") {
		score++
	}
	if score >= 2 {
		return Analysis{Kind: KindLog, Confidence: 0.82, Reasons: []string{"log/test/console markers"}, Stats: stats}, true
	}
	return Analysis{}, false
}

func mixedSignals(text string) []string {
	signals := make([]string, 0, 4)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			signals = appendSignal(signals, "fenced code")
		case strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["):
			signals = appendSignal(signals, "json-like block")
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "# "):
			signals = appendSignal(signals, "markdown-like prose")
		}
	}
	if reSeverity.MatchString(text) {
		signals = appendSignal(signals, "log severity")
	}
	return signals
}

func appendSignal(signals []string, signal string) []string {
	for _, existing := range signals {
		if existing == signal {
			return signals
		}
	}
	return append(signals, signal)
}
