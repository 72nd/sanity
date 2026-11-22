#ifndef SANITY_PDFMAC_H
#define SANITY_PDFMAC_H
#include <stdlib.h>

// All returned strings and buffers are malloc'd; the caller frees them.
// err is set (malloc'd) on failure.

// Page count (>=0) or -1; producer/creator are malloc'd or NULL; created is
// seconds since the Unix epoch and hasCreated says whether it is valid.
int sanity_meta(const char *path, char **producer, char **creator, double *created, int *hasCreated, char **err);

// Renders a page to an RGBA buffer (premultiplied, opaque white background).
unsigned char *sanity_render(const char *path, int page, double scale, int *w, int *h, char **err);

// Text layer of a page plus page size in points and the size in pixels of the
// largest embedded image on the page (0 when none).
char *sanity_page_text(const char *path, int page, double *pageW, double *pageH, int *imgW, int *imgH, char **err);

// Renders the page at scale and runs Vision OCR on it. Returns NULL with err
// NULL when OCR found nothing.
char *sanity_ocr_page(const char *path, int page, double scale, char **err);

// Warm-up: OCR of a tiny generated image with text. Returns 1 when Vision read it.
int sanity_ocr_available(void);

char *sanity_dates(const char *text);
#endif
