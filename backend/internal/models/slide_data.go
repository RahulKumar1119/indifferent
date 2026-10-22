package models

// QuestionSlideData is the renderer payload for a question or answer slide.
type QuestionSlideData struct {
	QuestionNumber int           `json:"questionNumber"`
	TotalQuestions int           `json:"totalQuestions"`
	QuestionText   string        `json:"questionText"`
	Options        []Option      `json:"options"`
	CorrectIndex   int           `json:"correctIndex"`
	CorrectIndices []int         `json:"correctIndices"`
	DesignOptions  DesignOptions `json:"designOptions,omitempty"`
}

// CountdownSlideData is the renderer payload for a countdown slide.
type CountdownSlideData struct {
	Seconds       int           `json:"seconds"`
	DesignOptions DesignOptions `json:"designOptions,omitempty"`
}

// OutroSlideData is the renderer payload for an outro slide.
type OutroSlideData struct {
	ChannelName   string        `json:"channelName"`
	DesignOptions DesignOptions `json:"designOptions,omitempty"`
}
