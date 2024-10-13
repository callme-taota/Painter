import hljs from "highlight.js"
import MarkdownIt from "markdown-it"

type renderCustomerFunc = (md: MarkdownIt) => void

type markdownToc = {
    level: number,
    content: string,
    anchor: string
}

type markdownTocList = markdownToc[]

export class PainterMarkdown {
    private content: string
    private md: MarkdownIt
    private customizeRenderList: renderCustomerFunc[] = [renderImage, renderLink]
    private tocList: markdownTocList = []

    constructor(content: string) {
        this.content = content
        this.md = new MarkdownIt({
            highlight: extendHighlight,
        })
        this.customizeRenderer()
        this.tocList = markdownToc(this.md, this.content)
    }

    private customizeRenderer() {
        for (let func of this.customizeRenderList) {
            func(this.md)
        }
    }

    createHTML(): string {
        return this.md.render(this.content)
    }

    get Toc() {
        return this.tocList
    }
}

const renderImage: renderCustomerFunc = (md: MarkdownIt) => {
    md.renderer.rules.image = (tokens, idx, options, env, self) => {
        const token = tokens[idx];
        const src = token.attrGet('src') || '';
        const alt = token.content || '';
        const title = token.attrGet('title') || '';
        return `
        <div class="md-image-container">
            <img class="md-image" src="${src}" alt="${alt}" ${title ? `title="${title}"` : ""} />
        </div>
        `;
    };
}

const renderLink: renderCustomerFunc = (md: MarkdownIt) => {
    md.renderer.rules.link = (tokens, idx, options, env, self) => {
        const token = tokens[idx];
        const href = token.attrGet('href');
        const title = token.attrGet('title') || '';
        const text = token.content;
        return `<a href="${href}" title="${title}" target="_blank" rel="noopener noreferrer">${text}</a>`;
    };
}

const extendHighlight = (str: string, lang: string) => {
    if (lang && hljs.getLanguage(lang)) {
        try {
            return '<pre><code class="hljs">' +
                hljs.highlight(lang, str, true).value +
                '</code></pre>';
        } catch (__) { }
    }

    return '';
}

const markdownToc = (md: MarkdownIt, content: string): markdownTocList => {
    const tokens = md.parse(content, {});
    const toc: markdownTocList = [];

    tokens.forEach((token, index) => {
        if (token.type === 'heading_open') {
            const level = parseInt(token.tag.replace('h', ''), 10);
            const nextToken = tokens[index + 1];
            const content = nextToken && nextToken.content ? nextToken.content : '';
            const anchor = content.toLowerCase().replace(/\s+/g, '-').replace(/[^\w-]+/g, '');
            if (content == "") {
                return
            }
            toc.push({ level, content, anchor });
        }
    });

    return toc;
}