package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"time"

	"github.com/jackc/pgx/v5"
)

type DemoReport struct {
	AuthorID     string `json:"author_id"`
	CreatedPosts int    `json:"created_posts"`
	PublicPosts  int    `json:"public_posts"`
}

// SeedDemo creates real, idempotent sample records. It deliberately does not
// create an owner, external identity, session, password, or authentication bypass.
func (a *App) SeedDemo(ctx context.Context) (DemoReport, error) {
	report := DemoReport{}
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return report, e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7284923151124)"); e != nil {
		return report, e
	}
	if _, e = tx.Exec(ctx, "SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE"); e != nil {
		return report, e
	}
	now := NowMillis()
	var author string
	e = tx.QueryRow(ctx, "INSERT INTO users(username,display_name,bio,role,created_at_ms,updated_at_ms) VALUES('__painter_demo_writer','Painter 示例作者','用于本地体验的示例内容，站长配置本地账号后即可开始自己的创作。','author',$1,$1) ON CONFLICT DO NOTHING RETURNING id::text", now).Scan(&author)
	if e == pgx.ErrNoRows {
		e = tx.QueryRow(ctx, "SELECT id::text FROM users WHERE username='__painter_demo_writer' AND role='author' AND disabled_at_ms IS NULL AND NOT EXISTS(SELECT 1 FROM oauth_accounts WHERE user_id=users.id) AND NOT EXISTS(SELECT 1 FROM local_accounts WHERE user_id=users.id) AND NOT EXISTS(SELECT 1 FROM sessions WHERE user_id=users.id)").Scan(&author)
	}
	if e != nil {
		return report, e
	}
	report.AuthorID = author
	p := Principal{ID: author, Role: "author"}
	tagIDs := map[string]string{}
	for _, name := range []string{"工程", "设计", "随笔", "Go", "产品设计", "写作", "系统架构", "随手记"} {
		var id string
		e = tx.QueryRow(ctx, `SELECT COALESCE(a.canonical_tag_id,t.id)::text FROM tags t LEFT JOIN tag_aliases a ON a.tag_id=t.id WHERE painter_taxonomy_key(t.name)=painter_taxonomy_key($1) ORDER BY COALESCE(a.canonical_tag_id,t.id) LIMIT 1`, name).Scan(&id)
		if e == pgx.ErrNoRows {
			e = tx.QueryRow(ctx, "INSERT INTO tags(name,created_at_ms,updated_at_ms) VALUES($1,$2,$2) RETURNING id::text", name, now).Scan(&id)
		}
		if e != nil {
			return report, e
		}
		tagIDs[name] = id
	}
	samples := []struct{ Title, Summary, Topic, Tag, Body string }{
		{"把博客变成一间愿意停留的书房", "关于个人博客的另一个想法：让文字成为主角，让阅读重新变得轻松。", "随笔", "写作", "## 为什么还要写个人博客\n\n信息流很快，文字可以慢一点。一个属于自己的地方，装得下尚未成熟的思考，也容得下很久以前写下的答案。\n\n> 好的阅读体验，从不给文字添堵开始。\n\n## 先写下来，再慢慢整理\n\n不用每篇都写成长文。清楚的标题、合适的段落、可靠的保存，已经能让创作轻松很多。\n\n- 写下具体的问题\n- 留住自己的推理过程\n- 把值得分享的部分公开\n\n## 这只是开始\n\n这篇是可删除的示例文章。使用站长为你配置的本地账号登录后，可以在创作台开始新的文章。"},
		{"一次发布，如何走完数据的全链路", "从编辑器里的最后一次保存，到读者看到正文，可靠性藏在每个小步骤里。", "工程", "系统架构", "## 不让自动保存改动线上文章\n\n草稿是工作副本，修订是不可变快照，发布指针选择读者当前能看到的版本。它们分别承担自己的责任。\n\n```go\ntx, err := pool.Begin(ctx)\nif err != nil { return err }\ndefer tx.Rollback(ctx)\n// 校验版本，写入修订与发布指针\nreturn tx.Commit(ctx)\n```\n\n## 原子边界要明确\n\n| 数据 | 提交时机 |\n| --- | --- |\n| 不可变修订 | 发布事务 |\n| 公开搜索投影 | 同一事务 |\n| 业务事件 | 同一事务 |\n\n提交失败不显示发布成功，响应丢失用原来的幂等键重试。"},
		{"液态玻璃之下，还是一张安静的纸", "在界面里保留一点光和空间，也把足够清楚的文字留给读者。", "设计", "产品设计", "## 材料服务于内容\n\n玻璃导航帮助区分浮动的操作与稳定的内容。正文、长列表和编辑区使用实色，阅读时眼睛不必一直追着背景变化。\n\n## 克制的几个具体选择\n\n1. 把字号与行高做对\n2. 给段落留出呼吸的空间\n3. 深色主题重新检查对比度\n4. 在减少透明效果时提供完整实色方案\n\n这些选择没有复杂的动画，却会改变每一次阅读的感受。"},
		{"从单体开始，给未来留下接口", "低资源和可扩展并不矛盾，清楚的模块边界比运行组件的数量更重要。", "工程", "Go", "## 一个应用，明确的边界\n\n内容、身份、媒体、搜索和统计在同一个 Go 应用中运行，PostgreSQL 管理持久事实。前端产物在构建时嵌入，服务器运行不需要 Node。\n\n```text\nHTTP → 应用用例 → 领域规则 → PostgreSQL / MediaStore\n```\n\n## 扩展从真实需求开始\n\n新的身份提供方通过接口加入，媒体可以切到对象存储，公开搜索也可以保留独立适配器。先把今天的需求完整做好，再为明天增加合适的实现。"},
		{"中文搜索，要从真实的两个字开始", "只验证英文单词不够，短词、中文、标点和未公开内容都应该有确定的行为。", "工程", "系统架构", "## 中文也是一等公民\n\n有时我们只记得「并发」两个字，有时还记得「并发编程」四个字。搜索需要让这些输入都给出真实、有边界的结果。\n\n## 字面匹配与公开快照\n\n长词使用 PostgreSQL 的 pg_trgm 索引候选；短词在公开投影上做明确的子串匹配。百分号 %、下划线 _ 和感叹号 ! 都是普通的输入。\n\n草稿里独有的词不能进入搜索，撤回的文章也应该同步消失。这是数据可见性的一部分。"},
	}
	for i, s := range samples {
		client := fmt.Sprintf("painter-demo-v1-%02d", i)
		var id string
		at := now - int64(i)*int64(24*time.Hour/time.Millisecond)
		e = tx.QueryRow(ctx, "INSERT INTO posts(author_user_id,client_draft_id,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$3) ON CONFLICT(author_user_id,client_draft_id) DO NOTHING RETURNING id::text", author, client, at).Scan(&id)
		if e == pgx.ErrNoRows {
			continue
		}
		if e != nil {
			return report, e
		}
		d := draftInput{Title: s.Title, Summary: s.Summary, Markdown: s.Body, TagIDs: []string{tagIDs[s.Topic], tagIDs[s.Tag]}, MediaIDs: []string{}}
		rendered, e := validateDraft(&d)
		if e != nil {
			return report, e
		}
		if e = contentCanonicalize(ctx, tx, &d); e != nil {
			return report, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO post_drafts(post_id,title,summary,markdown,updated_by,updated_at_ms,cover) VALUES($1,$2,$3,$4,$5,$6,$7)", id, d.Title, d.Summary, d.Markdown, author, at, coverJSON(d.Cover))
		if e != nil {
			return report, e
		}
		if e = contentSetRelations(ctx, tx, id, d); e != nil {
			return report, e
		}
		rid, e := contentMakeSnapshot(ctx, tx, p, id, "import", d, rendered, 1)
		if e != nil {
			return report, e
		}
		if _, e = tx.Exec(ctx, "UPDATE posts SET first_published_at_ms=$2,version=version+1 WHERE id=$1", id, at); e != nil {
			return report, e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO post_publications(post_id,revision_id,published_by,published_at_ms) VALUES($1,$2,$3,$4)", id, rid, author, at); e != nil {
			return report, e
		}
		if e = contentAudit(ctx, tx, p, "post.create", id, httptest.NewRequest("POST", "/cli/seed-demo", nil)); e != nil {
			return report, e
		}
		report.CreatedPosts++
	}
	if _, e = tx.Exec(ctx, "UPDATE site_profile SET tagline='一间存放思考与日常的数字书房。',about_markdown=$1,seo_description='Painter Next个人博客示例站点：写作、工程与设计。',updated_at_ms=$2 WHERE id=1 AND tagline=''", "# 关于这个空间\n\n这里是 Painter Next 的示例站点。文字、代码与设计，在这里慢慢生长。\n\n示例数据由本地CLI创建，可以在站点设置中替换品牌信息。登录使用站长配置的本地账号，示例作者没有登录凭据。", now); e != nil {
		return report, e
	}
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM public_posts").Scan(&report.PublicPosts); e != nil {
		return report, e
	}
	return report, tx.Commit(ctx)
}
