package com.clawbench.app;

import android.content.Context;
import android.util.Log;
import cn.jpush.android.api.NotificationMessage;
import cn.jpush.android.service.JPushMessageReceiver;

public class JPushReceiver extends JPushMessageReceiver {
    private static final String TAG = "ClawBench";

    @Override
    public void onNotifyMessageArrived(Context context, NotificationMessage message) {
        Log.i(TAG, "JPush notification arrived: " + message.msgId);
    }

    @Override
    public void onNotifyMessageOpened(Context context, NotificationMessage message) {
        Log.i(TAG, "JPush notification opened: " + message.msgId);
    }

    @Override
    public void onRegister(Context context, String registrationId) {
        Log.i(TAG, "JPush registered: " + registrationId);
    }

    @Override
    public void onConnected(Context context, boolean isConnected) {
        Log.i(TAG, "JPush connected: " + isConnected);
    }
}
