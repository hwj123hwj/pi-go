package ai

func CalculateCost(model Model, usage Usage) float64 {
	input := float64(usage.InputTokens) * model.Cost.InputPerMega / 1_000_000
	output := float64(usage.OutputTokens) * model.Cost.OutputPerMega / 1_000_000
	return input + output
}
