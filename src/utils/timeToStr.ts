export function numTimeToString(numTime: number): string {
    let t = numTime * 1000;
    let d = new Date(t);
    let y = d.getFullYear()
    let m = d.getMonth()
    let mm = m > 10 ? m : "0" + m
    let da = d.getDate()
    let dd = da > 10 ? da : "0" + da
    let h = d.getHours()
    let hh = h > 10 ? h : "0" + h
    let mi = d.getMinutes()
    let mimi = mi > 10 ? mi : "0" + mi
    return y + "-" + mm + "-" + dd + " " + hh + ":" + mimi
}

export function dateToString(d: string): string {
    let day = new Date(d)
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

    if (diff < oneDay) {
        return `${day.getHours()}:${day.getMinutes()}`;
    } else if (diff < oneDay * 3) {
        return `${Math.floor(diff / oneDay)}天前`;
    } else {
        return `${day.getFullYear()}-${day.getMonth() + 1}-${day.getDate()}`;
    }
}