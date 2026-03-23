export const getCsrfToken = (): string => {
    const match = document.cookie.match(/MMCSRF=([^;]+)/);
    return match ? match[1] : '';
};
