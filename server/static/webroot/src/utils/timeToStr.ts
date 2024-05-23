export function numTimeToString(numTime: number): string {
    let t = numTime * 1000;
    let d = new Date(t);
    let y = d.getFullYear()
    let m = d.getMonth() + 1
    let mm = m > 9 ? m : "0" + m
    let da = d.getDate()
    let dd = da > 9 ? da : "0" + da
    let h = d.getHours()
    let hh = h > 9 ? h : "0" + h
    let mi = d.getMinutes()
    let mimi = mi > 9 ? mi : "0" + mi
    return y + "-" + mm + "-" + dd + " " + hh + ":" + mimi
}

export function dateToString(d: string): string {
    let day = new Date(d)
    let t = day.getTime() / 1000
    return numTimeToString(t)
}

export function dateNow(): string {
    let day = new Date()
    let t = day.getTime() / 1000
    return numTimeToString(t)
}

export function dateToDescriptionN(d: number): string {
    d *= 1000
    return dateToDescription(d + "")
}

export function dateToDescription(d: string): string {
    let day = new Date(d)
    let now = new Date()
    let diff = Math.abs(now.getTime() - day.getTime())

    const oneDay = 86400000; // 24 * 60 * 60 * 1000

    if (diff < oneDay && day.toDateString() == now.toDateString()) {
        return `${day.getHours().toString().padStart(2, '0')}:${day.getMinutes().toString().padStart(2, '0')}`;
    } else if (diff < oneDay * 5) {
        return `${Math.floor(diff / oneDay)}天前`;
    } else {
        return `${day.getFullYear()}-${day.getMonth() + 1}-${day.getDate()}`;
    }
}

export function dateDiff(d: string): string {
    let day = new Date(d)
    let now = new Date()
    let diff = Math.abs(now.getTime() - day.getTime())

    const oneDay = 86400000; // 24 * 60 * 60 * 1000

    return `${Math.floor(diff / oneDay)}天前`;
}