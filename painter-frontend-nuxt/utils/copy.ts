export const copy = (text: any | string) => {
    if (navigator.clipboard) {
        navigator.clipboard.writeText(text);
    } else {
        if (typeof window !== 'undefined') {
            var textarea = document.createElement('textarea');
            document.body.appendChild(textarea);
            textarea.style.position = 'fixed';
            textarea.style.clip = 'rect(0 0 0 0)';
            textarea.style.top = '10px';
            textarea.value = text;
            textarea.select();
            document.execCommand('copy', true);
            document.body.removeChild(textarea);
        }
    }
}