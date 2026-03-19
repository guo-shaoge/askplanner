Title: Custom bot usage guide - Developer Guides - Documentation - Lark Developer

URL Source: https://open.larksuite.com/document/client-docs/bot-v3/use-custom-bots-in-a-group

Markdown Content:
Last updated on 2025-10-30

The contents of this article

A custom bot is a bot that can only be used in the current group chat. This type of robot can complete the message push by calling the webhook address in the current group chat without being reviewed by the tenant administrator. This article mainly introduces how to use custom robots.

## Precautions

*   You need to have a certain server-side development foundation, and realize the message push function by calling the webhook address of the custom robot by request.

*   Custom bots are ready to use after being added to a group, no tenant admin approval required. This feature improves the portability of developing robots, but for the sake of tenant data security, it also limits the usage scenarios of custom robots, and custom robots do not have any data access rights.

*   If you want to implement robot group management, user information acquisition and other capabilities, it is recommended to refer to [Session-based interactive robot](https://open.larksuite.com/document/home/interactive-session-based-robot/introduction), through the robot application. For a comparison of the capabilities of custom bots and robot applications, see [Comparison of Capabilities](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/bot-v3/bot-overview#6994dff4).

*   The rate limits for custom bots are different from regular apps: for each single-tenant, single bot, the limit is 100 requests per minute and 5 requests per second. **It is recommended to avoid sending messages exactly at times like 10:00 or 17:30** to prevent system overload, which may cause a 11232 rate limiting error and result in message sending failures.

*   When sending a message, the request body data size cannot exceed 20 KB.

## Features

There are scenarios where enterprises automatically push messages to specific groups, for example, pushing monitoring alarms, sales leads, and operational content. In this type of scenario, you can add a custom robot to the group. The custom robot provides a webhook by default. By calling the webhook address from the server, the message notification from the external system can be pushed to the group in real time. The custom robot also includes security configurations in three dimensions: **custom keywords**, **IP whitelist** and **signature**, which is convenient for controlling the scope of webhook calls.

An example of a custom robot message push, as shown in the following figure:

![Image 1](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/54789262572a82fd0a54bbde654ee32d_B9QVvVnnTE.png?height=990&lazyload=true&maxWidth=600&width=1388)

## Add a custom bot to the group

### Procedure

1.   Invite custom robots into the group.

    1.   Enter the target group, click the More button in the upper right corner of the group, and click **Settings**.

![Image 2](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/5ceaa87b8ac75dd0dd25681744055135_KnZgTObxzf.png?height=1530&lazyload=true&maxWidth=600&width=2004) 
    2.   On the **Settings** interface on the right, click **Bots**.

![Image 3](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/e079fd8715aa1c2053ed062573dd675f_ixQ2TVrf2p.png?height=1510&lazyload=true&maxWidth=600&width=2000) 
    3.   On the **Bots** interface, click **Add Bot**.

    4.   In the **Add bot** dialog box, find **Custom Bot**, and **Add**.

![Image 4](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/d5b8b253871a6ea888c69d9d63360236_J0M0Dl8uxZ.png?height=340&lazyload=true&maxWidth=600&width=1186) 
    5.   Set a name and description for the custom bot, and click **Add**.

2.   Obtain the webhook address of the custom bot and click **Finish**.

    1.   After successfully adding the robot, check the **Webhook URL** corresponding to the robot. The address format is as follows:

`https://open.larksuite.com/open-apis/bot/v2/hook/xxxxxxxxxxxxxxxxxx`
    2.   **Please keep this webhook address properly**, and do not publish it on Gitlab, blogs and other publicly available websites, so as to avoid being maliciously called to send spam after the address is leaked.

![Image 5](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/40274dc920405ee49231db2d877045a9_8j0WBoVIHa.png?height=888&lazyload=true&maxWidth=600&width=1194) 

3.   Test calling the webhook address of the custom robot to send a message to the group it belongs to.

    1.   Initiate an HTTP POST request to the webhook address in any way.

You need to have a certain server-side development foundation, and call the webhook address through the server-side HTTP POST request. Taking the curl command as an example, the request example is as follows. You can execute the following command through the terminal of the macOS system or the console application of the Windows system to test.

`curl -X POST -H "Content-Type: application/json" \    -d '{"msg_type":"text","content":{"text":"request example"}}' \    https://open.larksuite.com/open-apis/bot/v2/hook/****`
Example command description:

        *   Request method: `POST`

        *   Request header: `Content-Type: application/json`

        *   Request body: `{"msg_type":"text","content":{"text":"request example"}}`

        *   webhook address: `https://open.larksuite.com/open-apis/bot/v2/hook/****` is an example value, you need to replace it with the real webhook address of your custom robot when actually calling.

When sending a request to a custom robot, it supports sending various message types such as text, rich text, group business card, and message card. For request descriptions of various message types, see [Description of supported message types](https://open.larksuite.com/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN#1b70f1fa).

After executing the command:

        *   If the request is successful, the command line will echo the following information.

`{         "StatusCode": 0, //Redundant field, for compatibility with stock history logic, not recommended         "StatusMessage": "success", //Redundant field, for compatibility with stock history logic, not recommended         "code": 0,         "data": {},         "msg": "success"}`
        *   If the request body format is incorrect, the following information will be returned.

`{          "code": 9499,          "msg": "Bad Request",          "data": {}}`

You can check whether there is a problem with the request body through the following instructions.

        *   Whether the content format of the request body is consistent with the sample codes of each message type.

        *   The request body size cannot exceed 20K.

    2.   After the command is executed, enter the group where the custom robot is located to view the test message.

![Image 6](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/e62c4150ba6d01fd9dc3f990870c6423_2tEvMoxx6o.png?height=214&lazyload=true&maxWidth=600&width=1182) 

### Next Steps

After successfully adding a custom robot, it is recommended that you add security settings for the custom robot to ensure the security of the robot receiving requests. For details, see [Add security settings for custom robots](https://open.larksuite.com/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN#ddf40249).

## Add security settings for custom bots

After adding a custom bot to a group, you can add security settings for the bot. Security settings are used to protect custom robots from being called maliciously. For example, when the webhook address is leaked due to improper storage, it may be called by malicious developers to send spam. By adding security settings, the robot can only be called successfully if the conditions of the security settings are met.

Currently provided security settings are as follows:

*   We strongly recommend adding security settings to custom bots for extra security.

*   In the same custom bot, you can set one or more methods.

*   Custom keywords: Only messages containing at least one keyword can be sent successfully.

*   IP whitelist: Only IP addresses in the whitelist can successfully request webhook to send messages.

*   Signature Verification: Set the signature. The sent request must pass the signature verification before it can successfully request the webhook to send the message.

### Method 1: Set custom keywords

1.   In the group settings, open the robot list, find the custom robot and click to enter the configuration page.

![Image 7](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/373972b7d70164b9a974b9a10e04211b_WwwYuYQVkJ.png?height=1126&lazyload=true&maxWidth=600&width=1998) 
2.   In the **Security settings** area, select **Set Keywords**.

![Image 8](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/aacc78ebf90498579d0e8c60f931659e_y8rCRc2Fxx.png?height=1134&lazyload=true&maxWidth=600&width=1518) 
3.   Add keywords in the input box.

    *   You can set up to 10 keywords at the same time, and use the Enter key to space between multiple keywords. When set, only messages containing at least one keyword will be sent successfully.

For example, if the keywords are set to "Application Alert" and "Project Update", at least one of the keywords "Application Alert" or "Project Update" must be included in the requested webhook information.

    *   After setting keywords, if the custom keyword verification fails when sending a request, the following information will be returned.

`// Keyword validation failed{     "code": 19024,     "msg": "Key Words Not Found"}`

4.   Click **Save** to make the configuration take effect.

### Method 2: Set IP whitelist

1.   In the group settings, open the robot list, find the custom robot and click to enter the configuration page.

![Image 9](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/373972b7d70164b9a974b9a10e04211b_WwwYuYQVkJ.png?height=1126&lazyload=true&maxWidth=600&width=1998) 
2.   In the **Security settings** area, select **Set IP Whitelist**.

![Image 10](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/6994c815551a51056876a2ac8e381f24_jG3zHuOg4H.png?height=1130&lazyload=true&maxWidth=600&width=1510) 
3.   Add the IP address in the input box.

    *   Support adding IP addresses or address segments, up to 10 can be set, using the Enter key for intervals. Segment input is supported, such as `123.12.1.*` or `123.1.1.1/24`. When set, the robot webhook address will only handle requests from IP whitelisted ranges.

    *   After setting the IP whitelist, when the IP address outside the whitelist requests webhook, the verification will fail and the following information will be returned.

`// IP verification failed{     "code": 19022,     "msg": "Ip Not Allowed"}`

4.   Click **Save** to make the configuration take effect.

### Method 3: Set signature verification

1.   In the group settings, open the robot list, find the custom robot and click to enter the configuration page.

![Image 11](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/373972b7d70164b9a974b9a10e04211b_WwwYuYQVkJ.png?height=1126&lazyload=true&maxWidth=600&width=1998) 
2.   In the **Security settings** area, select **Set signature verification**.

After selecting the full name verification, the system has provided a secret key by default. You can also click **Reset** to change the key.

![Image 12](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/dd31d4c51aa271f21d247c1e8fc638c7_a9C2kvIIqc.png?height=1138&lazyload=true&maxWidth=600&width=1514) 
3.   Click **Copy** to copy the key.

4.   Click **Save** to make the configuration take effect.

5.   Compute the signature string.

After setting the signature verification, sending requests to the webhook requires signature verification to ensure that the source is trusted. The verified signature needs to be encrypted by the timestamp and the secret key algorithm, that is, `timestamp + "\n" + key` is used as the signature string, and the HmacSHA256 algorithm is used to calculate the signature, and then Base64 encoding.

`timestamp` refers to the timestamp no more than 1 hour (3600 seconds) from the current time, time unit: s. For example, 1599360473.

This article provides the following code samples in different languages to calculate the signature string.

    *   Java sample code

`package sign;import javax.crypto.Mac;import javax.crypto.spec.SecretKeySpec;import java.nio.charset.StandardCharsets;import java.security.InvalidKeyException;import java.security.NoSuchAlgorithmException;import org.apache.commons.codec.binary.Base64;public class SignDemo {public static void main(String[] args) throws NoSuchAlgorithmException, InvalidKeyException {String secret = "demo";int timestamp = 100;    System.out.printf("sign: %s", GenSign(secret, timestamp));}private static String GenSign(String secret, int timestamp) throws NoSuchAlgorithmException, InvalidKeyException {//Take timestamp+"\n"+ key as signature stringString stringToSign = timestamp + "\n" + secret;//Use the HmacSHA256 algorithm to calculate the signatureMac mac = Mac. getInstance("HmacSHA256");    mac.init(new SecretKeySpec(stringToSign.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));byte[] signData = mac.doFinal(new byte[]{});return new String(Base64. encodeBase64(signData));  }}`
    *   Go sample code

`func GenSign(secret string, timestamp int64) (string, error) {//timestamp + key do sha256, then base64 encode   stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + secretvar data []byte   h := hmac.New(sha256.New, []byte(stringToSign))   _, err := h. Write(data)if err != nil {return "", err   }   signature := base64.StdEncoding.EncodeToString(h.Sum(nil))return signature, nil}`
    *   Python sample code

`import hashlibimport base64import hmacdef gen_sign(timestamp, secret):# Splicing timestamp and secret  string_to_sign = '{}\n{}'.format(timestamp, secret)  hmac_code = hmac.new(string_to_sign.encode("utf-8"), digestmod=hashlib.sha256).digest()# Perform base64 processing on the result  sign = base64.b64encode(hmac_code).decode('utf-8')return sign`

6.   Get the signature string.

Taking the Java sample code as an example, after obtaining the current timestamp and key, run the program to obtain the signature string.

![Image 13](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/9578d3d77db27be88a572f0e67247607_M9dq4bZ89d.png?height=1234&lazyload=true&maxWidth=600&width=2090) 
After obtaining the signature string, when sending a request to the webhook, you need to add the timestamp (timestamp) and signature string (sign) field information. A sample configuration is shown below.

`// Send a text message after enabling signature verification{        "timestamp": "1599360473", // Timestamp.        "sign": "xxxxxxxxxxxxxxxxxxxxx", // The obtained signature string.        "msg_type": "text",        "content": {                "text": "request example"        }}`
If the verification fails when sending the request, you can troubleshoot the problem through the following instructions.

    *   The timestamp used is more than 1 hour from the time the request was sent, and the signature has expired.

    *   The server time has a large deviation from the standard time, causing the signature to expire. Please pay attention to check and adjust your server time.

    *   If the verification fails due to signature mismatch, the following information will be returned.

`// signature verification failed{        "code": 19021,        "msg": "sign match fail or timestamp is not within one hour from current time"}`

## Delete custom bot

In the **Settings** of the Lark group, open the **Bots** list, find the custom robot that needs to be deleted, and click the delete icon on the right side of the card.

![Image 14](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/40c4531cf734aed297c00abf52a0d038_mEbOt7DpYX.png?height=362&lazyload=true&maxWidth=400&width=638)

## Description of supported message types

When sending a POST request to a custom robot webhook address, the supported message formats include **text**, **rich text**, **picture message** and **group business card**, etc. This chapter introduces each message Type request format and display effect.

### Send text message

#### Request message body example

`{     "msg_type": "text",     "content": {         "text": "new update notification"     }}`
#### Realize the effect

![Image 15](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/cb3244b88ddab701b8a9e283fdb9a942_QcfqtSLDaS.png?height=188&lazyload=true&maxWidth=600&width=1182)

#### Parameter Description

*   The value of the parameter `msg_type` is the mapping relationship of the corresponding message type, and the corresponding value of `msg_type` for text messages is `text`.

*   The parameter `content` contains the message content, and the description of the message content parameters of the text message is shown in the table below.

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| text | string | is | Test content | Text content. | 

#### @ usage for text messages

`// @ single user<at user_id="ou_xxx">Name</at>// @ all<at user_id="all">everyone</at>`
*   @ Single user, the `user_id` field needs to be filled with the user's [Open ID](https://open.larksuite.com/document/uAjLw4CM/ugTN1YjL4UTN24CO1UjN/trouble-shooting/how-to-obtain-openid), and must be a valid value, otherwise take The name display does not produce the actual @ effect.

*   @Everyone: The @Everyone function must be enabled for the group you belong to.

#### Text Message @ Usage Example

`{     "msg_type": "text",     "content": {         "text": "<at user_id="ou_xxx">Tom</at> new update reminder"     }}`
### Send rich text message

Rich text messages refer to compound text information including text, hyperlinks, icons and other text styles.

#### Request message body example

`{         "msg_type": "post",         "content": {                 "post": {                         "zh_cn": {                                 "title": "Project Update Notification",                                 "content": [                                         [{                                                         "tag": "text",                                                         "text": "Item has been updated: "                                                 },                                                 {                                                         "tag": "a",                                                         "text": "Please check",                                                         "href": "http://www.example.com/"                                                 },                                                 {                                                         "tag": "at",                                                         "user_id": "ou_18eac8********17ad4f02e8bbbb"                                                 }                                         ]                                 ]                         }                 }         }}`
#### Realize the effect

![Image 16](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/57b4de91afb7a608da131188cbbedf3a_2VyTL8siQQ.png?height=258&lazyload=true&maxWidth=600&width=1184)

#### Parameter Description

*   The value of the parameter `msg_type` is the mapping relationship of the corresponding message type, and the corresponding value of `msg_type` of the text message is `post`.

*   The parameter `content` contains the message content, and the description of the message content parameters of the text message is shown in the table below.

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| post | object | is | none | Rich text message. |
| ∟ zh_cn | object | yes | none | `zh_cn` and `en_us` are the Chinese and English configurations of the rich text respectively, and at least one language configuration must be included in the rich text message. For the description of the included parameters, see the "`zh_cn`, `en_us` Field Description Table" below. |
| ∟ en_us | object | yes | none | `zh_cn` and `en_us` are the Chinese and English configurations of the rich text respectively, and at least one language configuration must be included in the rich text message. For the description of the included parameters, see the "`zh_cn`, `en_us` Field Description Table" below. | 
*   `zh_cn`, `en_us` field description table.

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| title | string | No | Test title | The title of the rich text message. |
| content | []paragraph | is | [[{"tag": "text","text": "text content"}]] | rich text message content. It consists of multiple paragraphs, and each paragraph is a `[]` node, which contains several nodes. | 

#### Labels and parameter descriptions supported by rich text

**Text label: text**

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| text | string | is | Text content | Text content. |
| un_escape | boolean | No | false | Indicates whether to unescape decoding. The default value is false, and it can be left blank when unescape is not used. |

**Hyperlink label: a**

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| text | string | yes | test URL | the text content of the hyperlink. |
| href | string | Yes | https://open.larksuite.com | The default link address, you need to ensure the legitimacy of the link address, otherwise the message will fail to send. |

**@ label: at**

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| user_id | string | is | ou_18eac85d35a26****02e8bbbb | [Open ID] of the user (/ssl:ttdoc/home/user-identity-introduction/open-id). - When @ a single user, the `user_id` field must be a valid value. - When @everyone, fill in `all`. |
| user_name | string | No | Jian Li | User name. |

**Image Tag: img**

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| image_key | string | Yes | d640eeea-4d2f-4cb3-88d8-c96fa5**** | The unique identifier of the image. The image_key can be obtained through the [upload image](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/image/create) interface. |

### Send group business card

#### Request message body example

`{     "msg_type": "share_chat",     "content":{         "share_chat_id": "oc_f5b1a7eb27ae2****339ff"     }}`
#### Realize the effect

![Image 17](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/aa7d5ad9fdee97e5adfa45385df3fecf_BiQD9z2Bxv.png?height=386&lazyload=true&maxWidth=600&width=1184)

#### Parameter Description

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| share_chat_id | string | yes | oc_f5b1a7eb27ae2****339ff | group ID. For how to obtain it, please refer to [Group ID Description](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/chat-id-description). |

### send pictures

#### Request message body example

`{     "msg_type": "image",     "content":{         "image_key": "img_ecffc3b9-8f14-400f-a014-05eca1a4310g"     }}`
#### Realize the effect

![Image 18](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/fd816697043a37e98a5d32e3f699f05b_4ySJ4KgNVN.png?height=700&lazyload=true&maxWidth=600&width=1178)

#### Parameter Description

| **Field** | **Type** | **Required** | **Example Value** | **Description** |
| --- | --- | --- | --- | --- |
| image_key | string | Yes | img_ecffc3b9-8f14-400f-a014-05eca1a4310g | Image Key. The image_key can be obtained through the [upload image](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/image/create) interface. |

### Send message card

Lark Message Card is a lightweight message push application that can be built from various types of components such as buttons and pictures. For information about the design specifications and formats of Lark message cards, see [Card Structure Introduction](https://open.larksuite.com/document/ukTMukTMukTM/uEjNwUjLxYDM14SM2ATN).

#### Precautions

*   The message card sent by the custom robot only supports jumping to the URL through buttons and text links, and does not support [postback interaction](https://open.larksuite.com/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN#49904b71) that calls back information to the server after clicking.

*   If you need to @ a certain user in the message card, you need to pay attention: the custom robot only supports `open_id` (see [what is open_id](https://open.larksuite.com/document/home/user-identity-introduction/open-id) ) method, and does not support `email`, `user_id` and other methods.

*   When sending a message card, you need to replace the `content` string in the message body with the `card` structure.

#### Request message body example

`{     "msg_type": "interactive",     "card": {         "elements": [{                 "tag": "div",                 "text": {                         "content": "**West Lake**, located at No. 1 Longjing Road, Xihu District, Hangzhou City, Zhejiang Province, west of Hangzhou City, with a total area of 49 square kilometers, a catchment area of 21.22 square kilometers, and a lake area of 6.38 square kilometers km.",                         "tag": "lark_md"                 }         }, {                 "actions": [{                         "tag": "button",                         "text": {                                 "content": "More attractions introduction: Rose:",                                 "tag": "lark_md"                         },                         "url": "https://www.example.com",                         "type": "default",                         "value": {}                 }],                 "tag": "action"         }],         "header": {                 "title": {                         "content": "Today's travel recommendation",                         "tag": "plain_text"                 }         }     }}`
#### Realize the effect

![Image 19](https://sf16-sg.larksuitecdn.com/obj/open-platform-opendoc-sg/3ac47bfd769571d55c0ea868cc9d2f32_jPMgmLVnXX.png?height=336&lazyload=true&maxWidth=600&width=1186)

#### Related operations

*   You can use [message card builder tool](https://open.larksuite.com/document/ukTMukTMukTM/uYzM3QjL2MzN04iNzcDN/message-card-builder) to quickly generate a message card and obtain the data structure for use. The data structure generated from the tool corresponds to the request message body `card` field.

*   The message card building tool provides a template message capability. Adders of custom bots can send template messages they create through the bot. For the calling instructions of template messages, see [Message card builder](https://open.larksuite.com/document/ukTMukTMukTM/uYzM3QjL2MzN04iNzcDN/message-card-builder) Card Templates chapter.

## common problem

### How to implement @designated person, @all people?

You can use the `at` tag to achieve the @person effect in ordinary text messages (text), rich text messages (post), and message cards (interactive) sent by the robot. The specific request is as follows:

*   @people, @everyone in normal text messages (text)

    *   `at` tag specification

`// at specifies the user<at user_id="ou_xxx">Name</at> //The value must use the open_id in ou_xxxxx format to specify the person at// at everyone<at user_id="all">everyone</at>`
    *   Request body indication

`{        "msg_type": "text",        "content": {                "text": "<at user_id = \"ou_f43d7bf0bxxxxxxxxxxxxxxx\">Tom</at> text content"        }}`

*   @people、@Everyone in rich text message (post):

    *   `at` tag specification

`// at specifies the user{        "tag": "at",        "user_id": "ou_xxxxxxx", //The value must use the open_id in the format of ou_xxxxx to specify the person at        "user_name": "tom"}// at everyone{        "tag": "at",        "user_id": "all",//The value uses "all" to at everyone        "user_name": "Everyone"}`
    *   Request body indication

`{        "msg_type": "post",        "content": {                "zh_cn": {                        "title": "I am a title",                        "content": [                                [{                                                "tag": "text",                                                "text": "The first line:"                                        },                                        {                                                "tag": "at",                                                "user_id": "ou_xxxxxx",                                                "user_name": "tom"                                        }                                ],                                [{                                                "tag": "text",                                                "text": "The second line:"                                        },                                        {                                                "tag": "at",                                                "user_id": "all",                                                "user_name": "Everyone"                                        }                                ]                        ]                }        }}`

*   @people, @everyone in the message card (interactive)

    *   You can use the at person tag in the Markdown content of the message card, as shown below

`// at specifies the user<at id=ou_xxx></at> //The value must use the open_id in the format of ou_xxxxx to specify the person at// at everyone<at id=all></at>`
    *   The contents of `card` in the request body indicate:

`{        "msg_type": "interactive",        "card": {                "elements": [{                        "tag": "div",                        "text": {                                "content": "at everyone <at id=all></at> \n at designated person <at id=ou_xxxxxx></at>",                                "tag": "lark_md"                        }                }]        }}`

### How do I get the open_id needed to @designate a person?

A custom bot can send messages to its group (including external groups) without being approved by the tenant administrator. This flexibility in development also restricts custom robots from having any data access rights, otherwise the tenant's private information will be leaked without the administrator's knowledge.

Based on this premise, the custom robot itself cannot call the interface to obtain the user's open_id, or directly @people through the user's email address and mobile phone number (malicious developers may use this method to scan out group members' avatars, names and other private information). Therefore, you can develop a robot application, use the following controlled scheme to get the user's `open_id`, and then refer to [How to implement a robot@people](https://open.larksuite.com/document/ugTN1YjL4UTN24CO1UjN/uUzN1YjL1cTN24SN3UjN#acc98e1b), in the custom robot push @people in the message.

**Scheme 1: Check the user's `open_id` by email or mobile phone number**

1.   You need to [create a self-built application](https://open.larksuite.com/document/home/introduction-to-custom-app-development/self-built-application-development-process).

2.   Apply for permission for the application.

Obtain the user ID (contact:user.id:readonly) by phone number or email, create an app version, and submit it for release review.

3.   After the release of the version is approved, call the [Get User ID by Mobile Phone Number or Email](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/contact-v3/user/batch_get_id) interface, you can pass the user's mobile phone number or Mailbox gets the user's `open_id`.

**Scheme 2: Parse the message with @person content sent by the user to the robot, and obtain the open_id of the target user**

1.   You need to [create a self-built application](https://open.larksuite.com/document/home/introduction-to-custom-app-development/self-built-application-development-process).

2.   Complete the following application configuration operations.

    1.   Apply for permission for the application: obtain the single-chat message (im:message.p2p_msg) sent by the user to the robot, obtain and send single-chat and group messages (im:message).

    2.   Subscribe to the [Receive Message](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/events/receive) event under the **Message and Group** category.

    3.   Create an application version for this self-built application and submit it for release review.

3.   After the release of the version review, you can send @ a user's message in the single chat with the robot. Parse the returned content of the [receive message](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/events/receive) event, in which the `open_id` information of the @ user is reported in the message body.

Need help with a problem?

Need help with a problem?

The contents of this article

Feedback

OnCall

Collapse

Expand
