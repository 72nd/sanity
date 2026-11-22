#include <stdlib.h>
// all return malloc'd UTF-8 (caller frees) or NULL; err set on failure
int   pdf_page_count(const char *path);
// renders page (0-based) at scale (1.0 = 72dpi) to PNG bytes
unsigned char *pdf_render_png(const char *path, int page, double scale, size_t *outLen, char **err);
char *pdf_text(const char *path, int page /* -1 = all */);
char *ocr_png(const unsigned char *png, size_t n, int accurate, char **err);
char *detect_dates(const char *text);
int png_to_pdf(const char*png,const char*out);
