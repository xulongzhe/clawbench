# Keep JavaScript interface methods (inner class of MainActivity)
-keepclassmembers class com.clawbench.app.MainActivity$WebAppInterface {
    @android.webkit.JavascriptInterface <methods>;
}

# Keep JSch classes used for SSH tunneling
-keep class com.jcraft.jsch.** { *; }
-dontwarn com.jcraft.jsch.**

# OkHttp optional platform dependencies (not included in APK, suppress R8 warnings)
-dontwarn org.bouncycastle.jsse.**
-dontwarn org.conscrypt.**
-dontwarn org.openjsse.**

# okhttp-eventsource: keep event source handler classes used via reflection
-keep class com.launchdarkly.eventsource.** { *; }
