#import <Foundation/Foundation.h>
#import <PDFKit/PDFKit.h>
#import <Vision/Vision.h>
#import <AppKit/AppKit.h>
#include <string.h>
#include "pdfmac.h"

static char *dupstr(NSString *s) { return s ? strdup(s.UTF8String) : NULL; }

static void setErr(char **err, NSString *msg) {
  if (err) *err = dupstr(msg);
}

static PDFDocument *openDoc(const char *path, char **err) {
  PDFDocument *d = [[PDFDocument alloc] initWithURL:[NSURL fileURLWithPath:@(path)]];
  if (!d) setErr(err, @"cannot open PDF");
  return d;
}

int sanity_meta(const char *path, char **producer, char **creator, double *created, int *hasCreated, char **err) {
  @autoreleasepool {
    *producer = NULL; *creator = NULL; *hasCreated = 0; *created = 0;
    PDFDocument *d = openDoc(path, err);
    if (!d) return -1;
    NSDictionary *a = d.documentAttributes;
    id p = a[PDFDocumentProducerAttribute];
    id c = a[PDFDocumentCreatorAttribute];
    id t = a[PDFDocumentCreationDateAttribute];
    if ([p isKindOfClass:[NSString class]]) *producer = dupstr(p);
    if ([c isKindOfClass:[NSString class]]) *creator = dupstr(c);
    if ([t isKindOfClass:[NSDate class]]) { *created = [(NSDate *)t timeIntervalSince1970]; *hasCreated = 1; }
    return (int)d.pageCount;
  }
}

static CGContextRef newRGBAContext(size_t w, size_t h) {
  CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
  CGContextRef c = CGBitmapContextCreate(NULL, w, h, 8, 0, cs,
      (CGBitmapInfo)kCGImageAlphaPremultipliedLast | kCGBitmapByteOrderDefault);
  CGColorSpaceRelease(cs);
  return c;
}

// Renders into a new context (caller releases). NULL on failure.
static CGContextRef renderPage(PDFDocument *d, int page, double scale, size_t *ow, size_t *oh) {
  if (page < 0 || page >= (int)d.pageCount || scale <= 0) return NULL;
  PDFPage *p = [d pageAtIndex:page];
  CGPDFPageRef cp = p.pageRef;
  if (!cp) return NULL;
  CGRect box = CGPDFPageGetBoxRect(cp, kCGPDFCropBox);
  int rot = CGPDFPageGetRotationAngle(cp);
  size_t w = (size_t)ceil(box.size.width * scale), h = (size_t)ceil(box.size.height * scale);
  if (rot == 90 || rot == 270) { size_t t = w; w = h; h = t; }
  if (w == 0 || h == 0) return NULL;
  CGContextRef c = newRGBAContext(w, h);
  if (!c) return NULL;
  CGContextSetRGBFillColor(c, 1, 1, 1, 1);
  CGContextFillRect(c, CGRectMake(0, 0, w, h));
  CGAffineTransform t = CGPDFPageGetDrawingTransform(cp, kCGPDFCropBox, CGRectMake(0, 0, w, h), 0, true);
  CGContextConcatCTM(c, t);
  CGContextDrawPDFPage(c, cp);
  *ow = w; *oh = h;
  return c;
}

unsigned char *sanity_render(const char *path, int page, double scale, int *w, int *h, char **err) {
  @autoreleasepool {
    PDFDocument *d = openDoc(path, err);
    if (!d) return NULL;
    size_t cw, ch;
    CGContextRef c = renderPage(d, page, scale, &cw, &ch);
    if (!c) { setErr(err, @"render failed"); return NULL; }
    size_t stride = CGBitmapContextGetBytesPerRow(c);
    const unsigned char *src = CGBitmapContextGetData(c);
    unsigned char *out = malloc(cw * 4 * ch);
    for (size_t y = 0; y < ch; y++) memcpy(out + y * cw * 4, src + y * stride, cw * 4);
    CGContextRelease(c);
    *w = (int)cw; *h = (int)ch;
    return out;
  }
}

// Largest image XObject (pixels) in the page resources.
typedef struct { int w, h; } dims;
static void xoCallback(const char *key, CGPDFObjectRef obj, void *info) {
  dims *d = info;
  CGPDFStreamRef s;
  if (!CGPDFObjectGetValue(obj, kCGPDFObjectTypeStream, &s)) return;
  CGPDFDictionaryRef sd = CGPDFStreamGetDictionary(s);
  const char *sub = NULL;
  if (!CGPDFDictionaryGetName(sd, "Subtype", &sub) || strcmp(sub, "Image") != 0) return;
  CGPDFInteger w = 0, h = 0;
  CGPDFDictionaryGetInteger(sd, "Width", &w);
  CGPDFDictionaryGetInteger(sd, "Height", &h);
  if ((long)w * h > (long)d->w * d->h) { d->w = (int)w; d->h = (int)h; }
}

static void pageImageDims(CGPDFPageRef cp, int *iw, int *ih) {
  dims d = {0, 0};
  CGPDFDictionaryRef pd = CGPDFPageGetDictionary(cp), res = NULL, xo = NULL;
  if (pd && CGPDFDictionaryGetDictionary(pd, "Resources", &res) &&
      CGPDFDictionaryGetDictionary(res, "XObject", &xo))
    CGPDFDictionaryApplyFunction(xo, xoCallback, &d);
  *iw = d.w; *ih = d.h;
}

char *sanity_page_text(const char *path, int page, double *pageW, double *pageH, int *imgW, int *imgH, char **err) {
  @autoreleasepool {
    *pageW = *pageH = 0; *imgW = *imgH = 0;
    PDFDocument *d = openDoc(path, err);
    if (!d) return NULL;
    if (page < 0 || page >= (int)d.pageCount) { setErr(err, @"page out of range"); return NULL; }
    PDFPage *p = [d pageAtIndex:page];
    CGRect box = [p boundsForBox:kPDFDisplayBoxCropBox];
    *pageW = box.size.width; *pageH = box.size.height;
    if (p.pageRef) pageImageDims(p.pageRef, imgW, imgH);
    return dupstr(p.string ?: @"");
  }
}

static NSString *recognize(CGImageRef img, NSString **errOut) {
  VNRecognizeTextRequest *req = [[VNRecognizeTextRequest alloc] init];
  req.recognitionLevel = VNRequestTextRecognitionLevelAccurate;
  req.recognitionLanguages = @[@"de-DE", @"en-US"];
  req.usesLanguageCorrection = YES;
  VNImageRequestHandler *h = [[VNImageRequestHandler alloc] initWithCGImage:img options:@{}];
  NSError *e = nil;
  if (![h performRequests:@[req] error:&e]) {
    if (errOut) *errOut = [NSString stringWithFormat:@"%@ %ld %@", e.domain, (long)e.code, e.localizedDescription];
    return nil;
  }
  NSMutableString *out = [NSMutableString string];
  for (VNRecognizedTextObservation *o in req.results) {
    VNRecognizedText *t = [[o topCandidates:1] firstObject];
    if (t) [out appendFormat:@"%@\n", t.string];
  }
  return out;
}

char *sanity_ocr_page(const char *path, int page, double scale, char **err) {
  @autoreleasepool {
    PDFDocument *d = openDoc(path, err);
    if (!d) return NULL;
    size_t w, h;
    CGContextRef c = renderPage(d, page, scale, &w, &h);
    if (!c) { setErr(err, @"render failed"); return NULL; }
    CGImageRef img = CGBitmapContextCreateImage(c);
    CGContextRelease(c);
    NSString *e = nil;
    NSString *s = recognize(img, &e);
    CGImageRelease(img);
    if (!s) { setErr(err, e); return NULL; }
    if (s.length == 0) return NULL;
    return dupstr(s);
  }
}

int sanity_ocr_available(void) {
  @autoreleasepool {
    size_t w = 600, h = 160;
    CGContextRef c = newRGBAContext(w, h);
    if (!c) return 0;
    CGContextSetRGBFillColor(c, 1, 1, 1, 1);
    CGContextFillRect(c, CGRectMake(0, 0, w, h));
    NSGraphicsContext *g = [NSGraphicsContext graphicsContextWithCGContext:c flipped:NO];
    [NSGraphicsContext saveGraphicsState];
    [NSGraphicsContext setCurrentContext:g];
    NSDictionary *attrs = @{NSFontAttributeName: [NSFont systemFontOfSize:64],
                            NSForegroundColorAttributeName: [NSColor blackColor]};
    [@"Invoice 2024" drawAtPoint:NSMakePoint(20, 40) withAttributes:attrs];
    [NSGraphicsContext restoreGraphicsState];
    CGImageRef img = CGBitmapContextCreateImage(c);
    CGContextRelease(c);
    NSString *s = recognize(img, NULL);
    CGImageRelease(img);
    return s.length > 0 ? 1 : 0;
  }
}

char *sanity_dates(const char *text) {
  @autoreleasepool {
    if (!text) return NULL;
    NSError *e = nil;
    NSDataDetector *dd = [NSDataDetector dataDetectorWithTypes:NSTextCheckingTypeDate error:&e];
    if (!dd) return NULL;
    NSString *s = @(text);
    NSMutableString *out = [NSMutableString string];
    [dd enumerateMatchesInString:s options:0 range:NSMakeRange(0, s.length)
                      usingBlock:^(NSTextCheckingResult *r, NSMatchingFlags f, BOOL *stop) {
      [out appendFormat:@"%@\n", [s substringWithRange:r.range]];
    }];
    return dupstr(out);
  }
}
