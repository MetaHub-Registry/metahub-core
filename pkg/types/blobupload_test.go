package types

/*func Test_BU_WWW(t *testing.T) {
	uid0, _ := uuid.FromBytes([]byte("1234567890"))
	bu0 := BlobUpload{
		UUID: uid0,
	}
	w0 := httptest.NewRecorder()
	b0 := OCIBlobUpload{
		Location: "Loc0",
	}
	// WriteOCIAcceptedResponse
	bu0.WriteOCIAcceptedResponse(w0, &b0)
	assert.Equal(t, 202, w0.Code)
	assert.Equal(t, "Loc0", w0.Header().Get("Location"))
	assert.NotEqual(t, "", bu0.UUID.String())
	assert.Equal(t, bu0.UUID.String(), w0.Header().Get("Docker-Upload-UUID"))
	assert.Equal(t, "0", w0.Header().Get("Content-Length"))
	// WriteOCICreateResponse
	bu1 := BlobUpload{
		UUID: uid0,
	}
	w1 := httptest.NewRecorder()
	b1 := OCIBlobUpload{
		Location: "Loc1",
	}
	bu0.WriteOCICreateResponse(w1, &b1)
	assert.Equal(t, 201, w1.Code)
	assert.Equal(t, "Loc1", w1.Header().Get("Location"))
	assert.NotEqual(t, "", bu1.UUID.String())
	assert.Equal(t, bu1.UUID.String(), w1.Header().Get("Docker-Upload-UUID"))
	assert.Equal(t, "0", w1.Header().Get("Content-Length"))
	// WriteOCIPatchResponse
	bu2 := BlobUpload{
		UUID: uid0,
	}
	w2 := httptest.NewRecorder()
	b2 := OCIBlobUpload{
		Location: "Loc2",
	}
	bu0.WriteOCIPatchResponse(w2, &b2)
	assert.Equal(t, 202, w2.Code)
	assert.Equal(t, "Loc2", w2.Header().Get("Location"))
	assert.NotEqual(t, "", bu2.UUID.String())
	assert.Equal(t, bu2.UUID.String(), w2.Header().Get("Docker-Upload-UUID"))
	assert.Equal(t, "10-10", w2.Header().Get("Range"))
	assert.Equal(t, "0", w2.Header().Get("Content-Length"))
	// WriteOCIPutResponse
	bu3 := BlobUpload{
		UUID: uid0,
	}
	w3 := httptest.NewRecorder()
	b3 := OCIBlobUpload{
		Location: "Loc3",
		Digest:   "sha256:1234567890",
	}
	bu0.WriteOCIPutResponse(w3, &b3)
	assert.Equal(t, 201, w3.Code)
	assert.Equal(t, "Loc3", w3.Header().Get("Location"))
	assert.NotEqual(t, "", bu3.UUID.String())
	assert.Equal(t, "sha256:1234567890", w3.Header().Get("Docker-Content-Digest"))
	assert.Equal(t, "0", w3.Header().Get("Content-Length"))
	assert.Equal(t, "10-10", w2.Header().Get("Range"))

}
*/
