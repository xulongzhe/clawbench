package com.clawbench.app;

import android.app.Notification;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;

import androidx.annotation.Nullable;
import androidx.core.app.NotificationCompat;

/**
 * Foreground Service that keeps the app alive while AI is processing.
 *
 * When the user sends a chat message, the WebView JS bridge calls startKeepAlive(),
 * which starts this service as a foreground service with a persistent notification.
 * This prevents Android from killing the app process while waiting for AI response,
 * ensuring SSE stream and TTS audio playback survive screen-off / app-backgrounding.
 *
 * The service is stopped when the AI finishes replying (done/cancelled event)
 * via the stopKeepAlive() JS bridge call.
 */
public class ChatKeepAliveService extends Service {

    private static final int NOTIFICATION_ID = 1;
    private static boolean isRunning = false;

    public static boolean isRunning() {
        return isRunning;
    }

    /**
     * Start the keep-alive foreground service.
     */
    public static void start(Context context) {
        if (isRunning) return;
        Intent intent = new Intent(context, ChatKeepAliveService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent);
        } else {
            context.startService(intent);
        }
    }

    /**
     * Stop the keep-alive foreground service.
     */
    public static void stop(Context context) {
        if (!isRunning) return;
        Intent intent = new Intent(context, ChatKeepAliveService.class);
        context.stopService(intent);
    }

    @Override
    public void onCreate() {
        super.onCreate();
        isRunning = true;
        startForeground(NOTIFICATION_ID, buildNotification());
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        // Ensure we're in foreground
        if (!isRunning) {
            isRunning = true;
            startForeground(NOTIFICATION_ID, buildNotification());
        }
        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        isRunning = false;
        stopForeground(true);
        super.onDestroy();
    }

    @Nullable
    @Override
    public IBinder onBind(Intent intent) {
        return null; // Not using bound service
    }

    private Notification buildNotification() {
        Intent notificationIntent = new Intent(this, MainActivity.class);
        PendingIntent pendingIntent = PendingIntent.getActivity(
                this, 0, notificationIntent,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
        );

        return new NotificationCompat.Builder(this, "clawbench_chat")
                .setContentTitle(getString(R.string.keep_alive_notification_title))
                .setContentText(getString(R.string.keep_alive_notification_text))
                .setSmallIcon(R.drawable.ic_notification)
                .setContentIntent(pendingIntent)
                .setOngoing(true)
                .setSilent(true)
                .build();
    }
}
