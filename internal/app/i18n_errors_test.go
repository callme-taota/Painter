package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalizedAPIErrorFormatsSourceWithoutChangingContract(t *testing.T) {
	values := []any{"name", "作者 {count} <script>"}
	err := ET(409, "EXAMPLE_CONFLICT", "事件缺少属性 {name}", values...)
	values[1] = "should not change the retained values"
	var original *APIError
	if !errors.As(err, &original) {
		t.Fatal("expected APIError")
	}
	original.Details = map[string]any{"name": "作者", "value": "资源不存在"}
	wantChinese := "事件缺少属性 作者 {count} <script>"
	if original.Message != wantChinese || original.Error() != "EXAMPLE_CONFLICT: "+wantChinese {
		t.Fatal("Chinese caller/log contract changed", original)
	}
	w := httptest.NewRecorder()
	w.Header().Set("Content-Language", "en")
	w.Header().Set("X-Request-ID", "i18n-unit-request")
	Error(w, fmt.Errorf("wrapped: %w", err))
	if w.Code != 409 {
		t.Fatal("status changed", w.Code)
	}
	var response struct {
		Error map[string]any `json:"error"`
	}
	if decodeErr := json.Unmarshal(w.Body.Bytes(), &response); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	got := response.Error
	if got["code"] != "EXAMPLE_CONFLICT" || got["request_id"] != "i18n-unit-request" || got["message"] != "The event is missing a property: 作者 {count} <script>" {
		t.Fatal("message interpolation changed opaque data or error metadata", got)
	}
	details, ok := got["details"].(map[string]any)
	if !ok || details["name"] != "作者" || details["value"] != "资源不存在" {
		t.Fatal("details were translated", got)
	}
	if len(got) != 4 || strings.Contains(w.Body.String(), "messageSource") || strings.Contains(w.Body.String(), "messageArgs") || strings.Contains(w.Body.String(), "<script>") {
		t.Fatal("internal localization fields or unescaped markup leaked", w.Body.String())
	}
	if original.Message != wantChinese || original.RequestID != "" {
		t.Fatal("response localization mutated original error", original)
	}
	if got := localeErrorMessage(original, defaultLocale); got != wantChinese {
		t.Fatal("default language changed", got)
	}
}

func TestLocalizedAPIErrorPreservesPlainValuesAndFallback(t *testing.T) {
	err := ET(400, "VALIDATION_FAILED", "事件属性无效 {name}", "name", "作者").(*APIError)
	if got := localeErrorMessage(err, "en"); got != "Invalid event property: 作者" {
		t.Fatal("plain user value was translated", got)
	}
	unknown := ET(400, "VALIDATION_FAILED", "未翻译消息 {name}", "name", "作者").(*APIError)
	if got := localeErrorMessage(unknown, "en"); got != "未翻译消息 作者" {
		t.Fatal("missing translation must fall back to source", got)
	}
	legacy := E(404, "NOT_FOUND", "资源不存在").(*APIError)
	if got := localeErrorMessage(legacy, "en"); got != "Resource not found" {
		t.Fatal("legacy E lost static localization", got)
	}
}

func TestLocalizedDynamicValidatorsUseCompleteMessages(t *testing.T) {
	now := NowMillis()
	event := analyticsEvent{Schema: 1, EventID: strings.Repeat("e", 16), Page: strings.Repeat("p", 16), Kind: "page_view.v1", ClientAt: now, Props: map[string]json.RawMessage{"作者": json.RawMessage(`1`)}}
	cases := []struct {
		err              error
		chinese, english string
	}{
		{event.validate(now), "事件包含未知属性 作者", "The event contains an unknown property: 作者"},
		{validateMomentTextField(strings.Repeat("a", MomentTextLimit+1)), fmt.Sprintf("动态最多%d个字符，空格与换行计入字数", MomentTextLimit), fmt.Sprintf("Updates allow up to %d characters, including spaces and line breaks", MomentTextLimit)},
	}
	_, missing := analyticsPropInt(&analyticsEvent{Props: map[string]json.RawMessage{}}, "作者", 0, 1)
	cases = append(cases, struct {
		err              error
		chinese, english string
	}{missing, "事件缺少属性 作者", "The event is missing a property: 作者"})
	_, invalid := analyticsPropInt(&analyticsEvent{Props: map[string]json.RawMessage{"作者": json.RawMessage(`"invalid"`)}}, "作者", 0, 1)
	cases = append(cases, struct {
		err              error
		chinese, english string
	}{invalid, "事件属性无效 作者", "Invalid event property: 作者"})
	for _, tc := range cases {
		var api *APIError
		if !errors.As(tc.err, &api) || api.Status != 400 || api.Code != "VALIDATION_FAILED" {
			t.Fatal("validator error contract changed", tc.err)
		}
		if api.Message != tc.chinese || localeErrorMessage(api, "en") != tc.english {
			t.Fatalf("want %q / %q, got %q / %q", tc.chinese, tc.english, api.Message, localeErrorMessage(api, "en"))
		}
	}
}

func TestLocalizedCardReasonsAreExplicitNestedMessages(t *testing.T) {
	cases := []struct {
		err              error
		chinese, english string
	}{
		{contentCardInvalid("请输入完整的 HTTPS 网址"), "内容卡片无效：请输入完整的 HTTPS 网址", "Invalid content card: Enter a complete HTTPS URL"},
		{contentLayoutFields([]byte(`{"v":3,"type":"layout","root":{"kind":"panel","children":[]}}`)), "内容卡片无效：panel 需要1至12个子节点", "Invalid content card: panel requires 1–12 child nodes"},
		{validateContentLayout(contentCard{V: 3, Type: "layout", Root: &ContentLayoutNode{Kind: "panel"}}), "内容卡片无效：panel 需要1至12个子节点", "Invalid content card: panel requires 1–12 child nodes"},
		{contentLayoutRequired(map[string]json.RawMessage{}, "作者"), "内容卡片无效：组合布局缺少字段 作者", "Invalid content card: The layout is missing a field: 作者"},
	}
	for _, tc := range cases {
		var api *APIError
		if !errors.As(tc.err, &api) || api.Status != 400 || api.Code != "CARD_INVALID" {
			t.Fatal("card error contract changed", tc.err)
		}
		if api.Message != tc.chinese || localeErrorMessage(api, "en") != tc.english {
			t.Fatalf("want %q / %q, got %q / %q", tc.chinese, tc.english, api.Message, localeErrorMessage(api, "en"))
		}
	}
}
