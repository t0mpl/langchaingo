package selfquery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/outputparser"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/tools/queryconstructor"
	queryconstructor_parser "github.com/tmc/langchaingo/tools/queryconstructor/parser"
)

// main function to retrieve documents with a query prompt.
func (sqr Retriever) GetRelevantDocuments(ctx context.Context, query string) ([]schema.Document, error) {
	prompt, err := queryconstructor.GetQueryConstructorPrompt(queryconstructor.GetQueryConstructorPromptArgs{
		DocumentContents: sqr.DocumentContents,
		AttributeInfo:    sqr.MetadataFieldInfo,
		EnableLimit:      sqr.EnableLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("error load query constructor %w", err)
	}

	promptChain := *chains.NewLLMChain(
		sqr.LLM,
		prompt,
		chains.WithTemperature(0),
	)

	promptChain.OutputParser = outputparser.NewJSONMarkdown()

	result, err := promptChain.Call(ctx, map[string]any{
		"query": query,
	})
	if err != nil {
		return nil, err
	}

	var resultBytes []byte
	var output map[string]interface{}
	var ok bool

	if resultBytes, ok = result["text"].([]byte); !ok {
		return nil, fmt.Errorf("wrong type retuned by json markdown parser")
	}

	if err = json.Unmarshal(resultBytes, &output); err != nil {
		return nil, fmt.Errorf("wrong json retuned by json markdown parser")
	}

	var filters any
	var queryRefinedPrompt string

	if filter, ok := output["filter"].(string); ok && filter != "NO_FILTER" {
		if filters, err = sqr.parseFilter(filter); err != nil {
			return nil, err
		}
	}

	if refinedPrompt, ok := output["query"].(string); ok {
		queryRefinedPrompt = refinedPrompt
	}

	limit, _ := output["limit"].(int)

	if limit == 0 {
		limit = sqr.DefaultLimit
	}

	return sqr.Store.Search(ctx, queryRefinedPrompt, filters, limit)
}

func (sqr Retriever) parseFilter(filter string) (any, error) {
	var err error
	var structuredFilter *queryconstructor_parser.StructuredFilter
	if structuredFilter, err = queryconstructor_parser.Parse(filter); err != nil {
		return nil, fmt.Errorf("query constructor couldn't parse query %w", err)
	}

	return sqr.Store.Translate(*structuredFilter)
}
