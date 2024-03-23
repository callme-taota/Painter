export function numTimeToString(numTime) {
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

export function dateToString(d) {
    let day = new Date(d)
    let t = day.getTime() / 1000
    return numTimeToString(t)
}