package config

import "testing"

func TestRecommendationsOptionalConfiguration(t *testing.T) {
	t.Setenv("TASK_HUNTER_TOKEN", "test")
	t.Setenv("MEDIA_TOKEN", "test")
	t.Setenv("FEED_TOKEN", "test")
	t.Setenv("RECOMMENDATIONS_URL", "")
	t.Setenv("RECOMMENDATIONS_SERVICE_TOKEN", "")
	cfg, err := Load()
	if err != nil || cfg.Recommendations.URL != "" {
		t.Fatalf("must remain disabled without URL: %+v %v", cfg.Recommendations, err)
	}
	t.Setenv("RECOMMENDATIONS_URL", "http://recommendations:8088")
	if _, err := Load(); err == nil {
		t.Fatal("enabled service accepted without service token")
	}
	t.Setenv("RECOMMENDATIONS_SERVICE_TOKEN", " test-token ")
	cfg, err = Load()
	if err != nil || cfg.Recommendations.Token != "test-token" || cfg.Recommendations.Timeout <= 0 {
		t.Fatalf("invalid configured client: %+v %v", cfg.Recommendations, err)
	}
	for _, url := range []string{"ftp://recommendations", "relative", "http://user:secret@recommendations"} {
		t.Setenv("RECOMMENDATIONS_URL", url)
		if _, err := Load(); err == nil {
			t.Fatalf("accepted invalid URL %s", url)
		}
	}
	t.Setenv("RECOMMENDATIONS_URL", "http://recommendations:8088")
	t.Setenv("RECOMMENDATIONS_TIMEOUT", "-1s")
	if _, err := Load(); err == nil {
		t.Fatal("accepted negative timeout")
	}
}
