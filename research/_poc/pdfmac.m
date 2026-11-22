#import <Foundation/Foundation.h>
#import <PDFKit/PDFKit.h>
#import <Vision/Vision.h>
#import <AppKit/AppKit.h>
#include <string.h>
#include "pdfmac.h"

static char *dupstr(NSString *s){ return s ? strdup(s.UTF8String) : NULL; }

int pdf_page_count(const char *path){
  @autoreleasepool {
    PDFDocument *d=[[PDFDocument alloc] initWithURL:[NSURL fileURLWithPath:@(path)]];
    return d? (int)d.pageCount : -1;
  }
}

static CGImageRef renderPage(const char*path,int page,double scale){
  PDFDocument *d=[[PDFDocument alloc] initWithURL:[NSURL fileURLWithPath:@(path)]];
  if(!d||page>=(int)d.pageCount) return NULL;
  PDFPage *p=[d pageAtIndex:page];
  CGPDFPageRef cp=p.pageRef; if(!cp) return NULL;
  CGRect box=CGPDFPageGetBoxRect(cp,kCGPDFCropBox);
  int rot=CGPDFPageGetRotationAngle(cp);
  size_t w=ceil(box.size.width*scale), h=ceil(box.size.height*scale);
  if(rot==90||rot==270){size_t t=w;w=h;h=t;}
  CGColorSpaceRef cs=CGColorSpaceCreateDeviceRGB();
  CGContextRef c=CGBitmapContextCreate(NULL,w,h,8,0,cs,kCGImageAlphaPremultipliedLast);
  CGColorSpaceRelease(cs);
  CGContextSetRGBFillColor(c,1,1,1,1); CGContextFillRect(c,CGRectMake(0,0,w,h));
  CGAffineTransform t=CGPDFPageGetDrawingTransform(cp,kCGPDFCropBox,CGRectMake(0,0,w,h),0,true);
  CGContextConcatCTM(c,t);
  CGContextDrawPDFPage(c,cp);
  CGImageRef img=CGBitmapContextCreateImage(c);
  CGContextRelease(c);
  return img;
}

unsigned char *pdf_render_png(const char *path,int page,double scale,size_t *outLen,char **err){
  @autoreleasepool {
    CGImageRef img=renderPage(path,page,scale);
    if(!img){ *err=strdup("render failed"); return NULL; }
    NSBitmapImageRep *rep=[[NSBitmapImageRep alloc] initWithCGImage:img];
    CGImageRelease(img);
    NSData *png=[rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
    unsigned char *b=malloc(png.length); memcpy(b,png.bytes,png.length); *outLen=png.length; return b;
  }
}

char *pdf_text(const char *path,int page){
  @autoreleasepool {
    PDFDocument *d=[[PDFDocument alloc] initWithURL:[NSURL fileURLWithPath:@(path)]];
    if(!d) return NULL;
    if(page<0) return dupstr(d.string?:@"");
    if(page>=(int)d.pageCount) return NULL;
    return dupstr([d pageAtIndex:page].string?:@"");
  }
}

char *ocr_png(const unsigned char *png,size_t n,int accurate,char **err){
  @autoreleasepool {
    NSData *data=[NSData dataWithBytes:png length:n];
    VNRecognizeTextRequest *req=[[VNRecognizeTextRequest alloc] init];
    req.recognitionLevel= accurate? VNRequestTextRecognitionLevelAccurate : VNRequestTextRecognitionLevelFast;
    req.recognitionLanguages=@[@"de-DE",@"en-US"];
    req.usesLanguageCorrection=YES;
    VNImageRequestHandler *h=[[VNImageRequestHandler alloc] initWithData:data options:@{}];
    NSError *e=nil;
    if(![h performRequests:@[req] error:&e]){ *err=dupstr([NSString stringWithFormat:@"%@ %ld %@",e.domain,(long)e.code,e.localizedDescription]); return NULL; }
    NSMutableString *out=[NSMutableString string];
    for(VNRecognizedTextObservation *o in req.results){
      VNRecognizedText *t=[[o topCandidates:1] firstObject]; if(t) [out appendFormat:@"%@\n",t.string];
    }
    return dupstr(out);
  }
}

char *detect_dates(const char *text){
  @autoreleasepool {
    NSError *e=nil;
    NSDataDetector *dd=[NSDataDetector dataDetectorWithTypes:NSTextCheckingTypeDate error:&e];
    if(!text) return NULL; NSString *s=@(text); NSMutableString *out=[NSMutableString string];
    [dd enumerateMatchesInString:s options:0 range:NSMakeRange(0,s.length) usingBlock:^(NSTextCheckingResult*r,NSMatchingFlags f,BOOL*stop){
      NSDateFormatter *df=[NSDateFormatter new]; df.dateFormat=@"yyyy-MM-dd HH:mm"; df.timeZone=r.timeZone?:[NSTimeZone localTimeZone];
      [out appendFormat:@"%@ -> %@\n",[s substringWithRange:r.range],[df stringFromDate:r.date]];
    }];
    return dupstr(out);
  }
}

int png_to_pdf(const char*png,const char*out){
  @autoreleasepool{
    NSImage *im=[[NSImage alloc] initWithContentsOfFile:@(png)]; if(!im) return 1;
    PDFDocument *d=[PDFDocument new]; [d insertPage:[[PDFPage alloc] initWithImage:im] atIndex:0];
    return [d writeToFile:@(out)]?0:2;
  }
}
