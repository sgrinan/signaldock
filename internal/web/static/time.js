document.querySelectorAll("time[data-local-time]").forEach((element) => {
    const date = new Date(element.dateTime);

    element.textContent = new Intl.DateTimeFormat(undefined, {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
    }).format(date);
});