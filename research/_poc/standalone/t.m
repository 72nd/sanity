#import <Vision/Vision.h>
#import <AppKit/AppKit.h>
int main(int argc,char**argv){@autoreleasepool{
 NSData*d=[NSData dataWithContentsOfFile:@(argv[1])];
 for(int lvl=0;lvl<2;lvl++){
 VNRecognizeTextRequest*r=[VNRecognizeTextRequest new]; r.recognitionLevel=lvl?VNRequestTextRecognitionLevelAccurate:VNRequestTextRecognitionLevelFast;
 NSError*e; NSArray*l=[r supportedRecognitionLanguagesAndReturnError:&e]; NSLog(@"lvl%d langs %@ err %@ rev %lu",lvl,l,e,(unsigned long)r.revision);
 r.usesCPUOnly=YES; r.recognitionLanguages=@[@"de-DE",@"en-US"];
 VNImageRequestHandler*h=[[VNImageRequestHandler alloc] initWithData:d options:@{}];
 BOOL ok=[h performRequests:@[r] error:&e]; NSLog(@"ok %d err %@ n=%lu",ok,e,(unsigned long)r.results.count);
 for(VNRecognizedTextObservation*o in r.results) NSLog(@"%@",[[o topCandidates:1].firstObject string]);}
}}
